package display

import (
	"fmt"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/xproto"
	"pkg.linuxdeepin.com/lib/dbus"
	"pkg.linuxdeepin.com/lib/log"
	"strings"
	"sync"
)

var (
	xcon, _ = xgb.NewConn()
	_       = initX11()

	Root           xproto.Window
	ScreenWidthMm  uint16
	ScreenHeightMm uint16

	LastConfigTimeStamp xproto.Timestamp

	MinWidth, MinHeight, MaxWidth, MaxHeight uint16

	logger = log.NewLogger("com.deepin.daemon.Display")
)

func initX11() bool {
	randr.Init(xcon)
	sinfo := xproto.Setup(xcon).DefaultScreen(xcon)
	Root = sinfo.Root
	ScreenWidthMm = sinfo.WidthInMillimeters
	ScreenHeightMm = sinfo.HeightInMillimeters
	LastConfigTimeStamp = xproto.Timestamp(0)

	ver, err := randr.QueryVersion(xcon, 1, 3).Reply()
	if err != nil {
		logger.Error("randr.QueryVersion error:", err)
		return false
	}
	if ver.MajorVersion != 1 || ver.MinorVersion != 3 {
		logger.Error("randr version is too low:", ver.MajorVersion, ver.MinorVersion, "this program require at least randr 1.3")
		return false
	}
	if err != nil {
		logger.Error("randr.GetSceenSizeRange failed :", err)
		return false
	}
	return true
}

var GetDisplay = func() func() *Display {
	dpy := &Display{}

	sinfo := xproto.Setup(xcon).DefaultScreen(xcon)
	dpy.setPropScreenWidth(sinfo.WidthInPixels)
	dpy.setPropScreenHeight(sinfo.HeightInPixels)
	GetDisplayInfo().update()
	dpy.setPropHasChanged(false)

	randr.SelectInputChecked(xcon, Root, randr.NotifyMaskOutputChange|randr.NotifyMaskOutputProperty|randr.NotifyMaskCrtcChange|randr.NotifyMaskScreenChange)

	return func() *Display {
		return dpy
	}
}()

type Display struct {
	Monitors    []*Monitor
	monitorLock sync.RWMutex

	ScreenWidth  uint16
	ScreenHeight uint16

	//used by deepin-dock/launcher/desktop
	Primary        string
	PrimaryRect    xproto.Rectangle
	PrimaryChanged func(xproto.Rectangle)

	DisplayMode   int16
	BuiltinOutput *Monitor

	HasChanged bool

	Brightness map[string]float64
	cfg        *ConfigDisplay
}

func (dpy *Display) lockMonitors() {
	dpy.monitorLock.Lock()
}
func (dpy *Display) unlockMonitors() {
	dpy.monitorLock.Unlock()
}
func (dpy *Display) rLockMonitors() {
	dpy.monitorLock.RLock()
}
func (dpy *Display) rUnlockMonitors() {
	dpy.monitorLock.RUnlock()
}

//plugging out an output wouldn't always rearrange screen allocation.
func (dpy *Display) fixOutputNotClosed(op randr.Output) {
	for _, present := range GetDisplayInfo().ListOutputs() {
		if op == present {
			return
		}
	}

	dpy.apply(true)
}

func (dpy *Display) listener() {
	for {
		e, err := xcon.WaitForEvent()
		if err != nil {
			continue
		}
		switch ee := e.(type) {
		case randr.NotifyEvent:
			switch ee.SubCode {
			case randr.NotifyCrtcChange:
			case randr.NotifyOutputChange:
				info := ee.U.Oc
				if info.Connection != randr.ConnectionConnected && info.Mode != 0 {
					randr.SetCrtcConfig(xcon, info.Crtc, xproto.TimeCurrentTime, LastConfigTimeStamp, 0, 0, 0, randr.RotationRotate0, nil)
				}
				if info.Mode == 0 || info.Crtc == 0 {
					dpy.fixOutputNotClosed(info.Output)
				}
			case randr.NotifyOutputProperty:
			}
		case randr.ScreenChangeNotifyEvent:
			dpy.setPropScreenWidth(ee.Width)
			dpy.setPropScreenHeight(ee.Height)

			GetDisplayInfo().update()

			if LastConfigTimeStamp < ee.ConfigTimestamp {
				LastConfigTimeStamp = ee.ConfigTimestamp
				if dpy.QueryCurrentPlanName() != dpy.cfg.CurrentPlanName {
					logger.Info("Detect New ConfigTimestmap, try reset changes")
					dpy.ResetChanges()
					dpy.SwitchMode(dpy.DisplayMode, dpy.cfg.Plans[dpy.cfg.CurrentPlanName].DefaultOutput)
				}
			}

			//sync Monitor's state
			for _, m := range dpy.Monitors {
				m.updateInfo()
			}

			//changePrimary will try set an valid primary if dpy.Primary invalid
			dpy.changePrimary(dpy.Primary, true)

			dpy.mapTouchScreen()
		}
	}
}

func (dpy *Display) mapTouchScreen() {
	for output, touchscreen := range dpy.cfg.MapToTouchScreen {
		runCodeAsync(fmt.Sprintf(`xinput map-to-output "%s" "%s"`, touchscreen, output))
	}
}

func (dpy *Display) AssociateTouchScreen(output string, touchscreen string) {
	//TODO: check name valid
	dpy.saveTouchScreen(output, touchscreen)
}

//The range of brightness value is 0.1~1.
//Generally speaking user can use media key to change brightness when the output
//supports backlight, but we can't rely on this assumption.
//If xrandr/acpi driver works, the value of zero is safety. But if the driver
//doesn't work well, ChangeBrightness has received a zero value and then the system
//will enter an unusable situation.
func (dpy *Display) ChangeBrightness(output string, v float64) error {
	if !validBrightnessValue(v) {
		//NOTO: don't use "if v < 0.1 || v > 1",  because there has some guy called NaN.
		return fmt.Errorf("Try change the brightness of %s to an invalid value(%v)", output, v)
	}

	op := GetDisplayInfo().QueryOutputs(output)
	if op == 0 {
		return fmt.Errorf("Chan't find the ", output, "output when change brightness")
	}

	if supportedBacklight(xcon, GetDisplayInfo().QueryOutputs(output)) {
		setBacklight(v)
	} else {
		setBrightness(xcon, op, v)
	}
	dpy.setPropBrightness(output, v)
	return nil

}

func (dpy *Display) ResetBrightness(output string) {
	if v, ok := LoadConfigDisplay(dpy).Brightness[output]; ok {
		dpy.SetBrightness(output, v)
	}
}
func (dpy *Display) SetBrightness(output string, v float64) error {
	if err := dpy.ChangeBrightness(output, v); err != nil {
		return err
	}
	dpy.saveBrightness(output, v)
	return nil
}

func (dpy *Display) JoinMonitor(a string, b string) error {
	dpy.lockMonitors()
	defer dpy.unlockMonitors()

	ms := dpy.cfg.Plans[dpy.cfg.CurrentPlanName].Monitors
	if ma, ok := ms[a]; ok {
		if mb, ok := ms[b]; ok {
			mc := mergeConfigMonitor(dpy, ma, mb)
			delete(dpy.cfg.Plans[dpy.cfg.CurrentPlanName].Monitors, a)
			delete(dpy.cfg.Plans[dpy.cfg.CurrentPlanName].Monitors, b)
			dpy.cfg.Plans[dpy.cfg.CurrentPlanName].Monitors[mc.Name] = mc

			var newMonitors []*Monitor
			for _, m := range dpy.Monitors {
				if m.Name != a && m.Name != b {
					newMonitors = append(newMonitors, m)
				}
			}
			newMonitors = append(newMonitors, NewMonitor(dpy, mc))
			dpy.setPropMonitors(newMonitors)
		} else {
			return fmt.Errorf("Can't find Monitor %s\n", b)
		}
	} else {
		return fmt.Errorf("Can't find Monitor %s\n", a)
	}
	return nil
}
func (dpy *Display) SplitMonitor(a string) error {
	dpy.lockMonitors()
	defer dpy.unlockMonitors()

	var monitors []*Monitor
	found := false
	for _, m := range dpy.Monitors {
		if m.Name == a {
			submonitors := m.split(dpy)
			if submonitors == nil {
				return fmt.Errorf("Can't find composited monitor: %s", a)
			}
			found = true
			monitors = append(monitors, submonitors...)
		} else {
			monitors = append(monitors, m)
		}
	}
	if found {
		dpy.setPropMonitors(monitors)
		return nil
	} else {
		return fmt.Errorf("Can't find composited monitor: %s", a)
	}
}
func (m *Monitor) split(dpy *Display) (r []*Monitor) {
	if !strings.Contains(m.Name, joinSeparator) {
		return
	}

	delete(dpy.cfg.Plans[dpy.QueryCurrentPlanName()].Monitors, m.Name)
	dpyinfo := GetDisplayInfo()
	for _, name := range strings.Split(m.Name, joinSeparator) {
		op := dpyinfo.QueryOutputs(name)
		if op == 0 {
			continue
		}
		mcfg, err := CreateConfigMonitor(dpy, op)
		if err != nil {
			logger.Error("Failed createconfigmonitor at split", err, name, mcfg)
			continue
		}
		dpy.cfg.Plans[dpy.QueryCurrentPlanName()].Monitors[name] = mcfg

		//TODO: check width/height value whether zero

		dpy.cfg.ensureValid(dpy)
		m := NewMonitor(dpy, mcfg)
		//TODO: change or set?
		m.SetMode((m.BestMode.ID))
		r = append(r, m)
	}
	return
}

func (dpy *Display) detectChanged() {
	if dpy.disableChanged() {
		dpy.setPropHasChanged(false)
		return
	}
	cfg := LoadConfigDisplay(dpy)
	if !cfg.ensureValid(dpy) {
		return
	}
	dpy.setPropHasChanged(!dpy.cfg.Compare(cfg))
}

func (dpy *Display) canBePrimary(name string) *Monitor {
	for _, m := range dpy.Monitors {
		if m.Name == name && m.Opened {
			return m
		}
	}
	return nil
}

func (dpy *Display) changePrimary(name string, effectRect bool) error {
	if m := dpy.canBePrimary(name); m != nil {
		dpy.setPropPrimary(name)
		if effectRect {
			dpy.setPropPrimaryRect(xproto.Rectangle{m.X, m.Y, m.Width, m.Height})
		}
		return nil
	}
	//the output whose name is `name` didn't exists or disabled,

	if dpy.canBePrimary(dpy.Primary) != nil {
		return fmt.Errorf("can't set %s as primary, current primary %s wouldn't be changed", name, dpy.Primary)
	}

	//try set an primary
	for _, m := range dpy.Monitors {
		if dpy.canBePrimary(m.Name) != nil {
			dpy.setPropPrimary(m.Name)
			if effectRect {
				dpy.setPropPrimaryRect(xproto.Rectangle{m.X, m.Y, m.Width, m.Height})
			}
			return fmt.Errorf("can't set %s as primary, and current parimary %s is invalid. fallback to %s",
				name, dpy.Primary, m.Name)
		}
	}
	logger.Error("can't find any valid primary", name)
	return fmt.Errorf("can't find any valid primary", name)
}

func (dpy *Display) SetPrimary(name string) error {
	if err := dpy.changePrimary(name, true); err != nil {
		return err
	}
	dpy.savePrimary(name)
	return nil
}

func (dpy *Display) disableChanged() bool {
	if len(dpy.Monitors) == 1 && !dpy.Monitors[0].IsComposited {
		return false
	}
	if dpy.DisplayMode == DisplayModeCustom {
		return false
	}
	return true
}

func (dpy *Display) Apply() {
	if dpy.disableChanged() {
		logger.Warning("Display.Apply only can be used in Custom DisplayMode.")
		return
	}
	dpy.apply(false)
}

func (dpy *Display) apply(auto bool) {
	dpy.rLockMonitors()
	defer dpy.rUnlockMonitors()

	code := "xrandr "
	for _, m := range dpy.Monitors {
		code += m.generateShell()
		if auto {
			code += " --auto"
		}

		if dpy.cfg.Primary == m.Name {
			code += " --primary"
		}
	}
	runCode(code)
}

func (dpy *Display) ResetChanges() {
	dpy.cfg = LoadConfigDisplay(dpy)
	dpy.setPropDisplayMode(dpy.cfg.DisplayMode)
	dpy.cfg.ensureValid(dpy)

	//must be invoked after LoadConfigDisplay(dpy)
	var monitors []*Monitor
	for _, mcfg := range dpy.cfg.Plans[dpy.cfg.CurrentPlanName].Monitors {
		m := NewMonitor(dpy, mcfg)
		m.updateInfo()
		monitors = append(monitors, m)
	}
	dpy.setPropMonitors(monitors)

	if err := dpy.changePrimary(dpy.cfg.Primary, true); err != nil {
		logger.Warning("chnagePrimary :", dpy.cfg.Primary, err)
	}

	//apply the saved configurations.
	dpy.Apply()
	dpy.setPropHasChanged(false)

	dpy.Brightness = make(map[string]float64)

	for name, v := range dpy.cfg.Brightness {
		dpy.ChangeBrightness(name, v)
	}
	//dpy.cfg.Brightness may doesn't contain all output, so we must
	//reset this output's brightness to 1
	for _, mcfg := range dpy.cfg.Plans[dpy.cfg.CurrentPlanName].Monitors {
		if _, ok := dpy.cfg.Brightness[mcfg.Name]; !ok {
			dpy.ChangeBrightness(mcfg.Name, 1)
		}
	}
}

func (dpy *Display) SaveChanges() {
	dpy.cfg.Save()
	dpy.detectChanged()
}

func (dpy *Display) Reset() {
	dpy.rLockMonitors()
	defer dpy.rUnlockMonitors()

	for _, m := range dpy.Monitors {
		dpy.SetBrightness(m.Name, 1)
		m.SetReflect(0)
		m.SetRotation(1)
		m.SetMode(m.BestMode.ID)
	}
	for _, m := range dpy.Monitors {
		for _, output := range m.Outputs {
			dpy.SetBrightness(output, 1)
		}
	}
	dpy.apply(true)
	dpy.SaveChanges()
}

func Start() {
	dpy := GetDisplay()
	err := dbus.InstallOnSession(dpy)
	if err != nil {
		logger.Error("Can't install dbus display service on session:", err)
		return
	}
	dpy.ResetChanges()
	dpy.SwitchMode(dpy.cfg.DisplayMode, dpy.cfg.Plans[dpy.cfg.CurrentPlanName].DefaultOutput)

	go dpy.listener()

	for _, m := range dpy.Monitors {
		m.updateInfo()
	}
	dpy.workaroundBacklight()
}

func (dpy *Display) QueryOutputFeature(name string) int32 {
	support := supportedBacklight(xcon, GetDisplayInfo().QueryOutputs(name))
	if support {
		return 1
	} else {
		return 0
	}
}
