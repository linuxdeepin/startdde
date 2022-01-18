package display

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/godbus/dbus"
	geoclue2 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.geoclue2"
	"pkg.deepin.io/lib/dbusutil"
)

func isValidColorTempMode(mode int32) bool {
	return mode >= ColorTemperatureModeNone && mode <= ColorTemperatureModeManual
}

// dbus 上导出的方法
func (m *Manager) setColorTempMode(mode int32) error {
	if !isValidColorTempMode(mode) {
		return errors.New("mode out of range, not 0 or 1 or 2")
	}
	m.setColorTempModeReal(mode)
	return nil
}

func (m *Manager) setColorTempModeReal(mode int32) {
	switch mode {
	case ColorTemperatureModeAuto: // 自动模式调节色温 启动服务
		m.redshiftRunner.start()

	case ColorTemperatureModeManual, ColorTemperatureModeNone:
		// manual 手动调节色温
		// none 恢复正常色温
		m.redshiftRunner.stop()
	}
	// 对于自动模式，也要先把色温设置为正常。
	m.setColorTempOneShot()
}

const (
	redshiftStateRunning = iota + 1
	redshiftStateStopped
)

type redshiftRunner struct {
	mu                 sync.Mutex
	state              int
	timer              *time.Timer
	process            *os.Process
	value              int
	cb                 func(value int)
	sysService         *dbusutil.Service
	geoAgentRegistered bool
}

func newRedshiftRunner() *redshiftRunner {
	sysService, err := dbusutil.NewSystemService()
	if err != nil {
		logger.Warning("new sys service failed:", err)
	}
	return &redshiftRunner{
		sysService: sysService,
	}
}

func (r *redshiftRunner) start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	logger.Debugf("redshiftRunner.start")

	if r.state == redshiftStateRunning {
		return
	}
	r.state = redshiftStateRunning

	err := r.registerGeoClueAgent()
	if err != nil {
		logger.Warning("register geoClue agent failed:", err)
	}

	cmd := exec.Command("redshift", "-m", "dummy", "-t", "6500:3500", "-r")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Warning("get stdout pipe failed:", err)
		r.state = redshiftStateStopped
		return
	}
	err = cmd.Start()
	if err != nil {
		logger.Warning("start redshift failed:", err)
		r.state = redshiftStateStopped
		return
	}

	r.process = cmd.Process
	r.value = 0

	go func() {
		logger.Debugf("redshift is running, pid: %v", r.process.Pid)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Bytes()
			logger.Debugf("line: %s", line)
			temp, ok := getTemperatureWithLine(line)
			if ok {
				logger.Debug("temp:", temp)
				r.updateValue(temp)
			}
		}
		err := scanner.Err()
		if err != nil {
			logger.Warning("scanner err:", err)
		}

		err = cmd.Wait()
		if err != nil {
			logger.Debugf("redshift cmd wait err: %v, errBuf: %s", err, errBuf.Bytes())
		}
		logger.Debug("redshift stopped")

		r.mu.Lock()
		r.state = redshiftStateStopped
		r.process = nil
		r.mu.Unlock()
	}()

	return
}

func (r *redshiftRunner) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state == redshiftStateStopped {
		return
	}

	logger.Debug("redshiftRunner.stop")
	if r.process != nil {
		err := r.process.Signal(os.Interrupt)
		if err != nil {
			logger.Warning("send signal interrupt to redshift process failed:", err)
		}
	}
	r.value = 0
}

func (r *redshiftRunner) updateValue(value int) {
	r.mu.Lock()
	if r.value == value {
		// no change
		r.mu.Unlock()
		return
	}
	r.value = value
	r.mu.Unlock()

	if r.cb != nil {
		r.cb(value)
	}
}

func (r *redshiftRunner) getValue() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.value
}

func getTemperatureWithLine(line []byte) (int, bool) {
	const prefix = "Temperature: "
	if bytes.HasPrefix(line, []byte(prefix)) {
		numStr := string(line[len(prefix):])
		num, err := strconv.Atoi(numStr)
		return num, err == nil
	}
	return 0, false
}

// dbus 上导出的方法
func (m *Manager) setColorTempValue(value int32) error {
	if m.ColorTemperatureMode.Get() != ColorTemperatureModeManual {
		return errors.New("current not manual mode, can not adjust color temperature by manual")
	}
	if !isValidColorTempValue(value) {
		return errors.New("value out of range")
	}
	m.setColorTempOneShot()
	m.ColorTemperatureManual.Set(value)
	return nil
}

func isValidColorTempValue(value int32) bool {
	return value >= 1000 && value <= 25000
}

func (m *Manager) getColorTemperatureValue() int {
	m.PropsMu.RLock()
	mode := m.ColorTemperatureMode.Get()
	manual := m.ColorTemperatureManual.Get()
	m.PropsMu.RUnlock()

	switch mode {
	case ColorTemperatureModeNone:
		return defaultTemperatureManual
	case ColorTemperatureModeManual:
		return int(manual)
	case ColorTemperatureModeAuto:
		return m.redshiftRunner.getValue()
	}

	return defaultTemperatureManual
}
var _setColorTempMu sync.Mutex

func (m *Manager) setColorTempOneShot() {
	_setColorTempMu.Lock()
	defer _setColorTempMu.Unlock()

	monitors := m.getConnectedMonitors()
	for _, monitor := range monitors {
		monitor.PropsMu.RLock()
		br := m.Brightness[monitor.Name]
		name := monitor.Name
		monitor.PropsMu.RUnlock()

		err := m.doSetBrightness(br, name)
		if err != nil {
			logger.Warning(err)
		}
	}
}

const (
	dbusIfcGeoClueAgent  = "org.freedesktop.GeoClue2.Agent"
	dbusPathGeoClueAgent = "/org/freedesktop/GeoClue2/Agent"
)

type geoClueAgent struct {
	MaxAccuracyLevel uint32
	methods *struct { //nolint
		AuthorizeApp	func() `in:"desktopId,reqAccuracyLevel" out:"authorized,allowedAccuracyLevel"`
	}
}

const (
	AccuracyLevelNone         = 0
	AccuracyLevelCountry      = 1
	AccuracyLevelCity         = 4
	AccuracyLevelNeighborhood = 5
	AccuracyLevelStreet       = 6
	AccuracyLevelExact        = 8
)

func (a *geoClueAgent) GetInterfaceName() string {
	return dbusIfcGeoClueAgent
}

func (a *geoClueAgent) AuthorizeApp(desktopId string, reqAccuracyLevel uint32) (authorized bool, allowedAccuracyLevel uint32, busErr *dbus.Error) {
	// 目前发现这个方法不会被调用。
	logger.Debugf("AuthorizeApp desktopId: %q, reqAccuracyLevel: %v", desktopId, reqAccuracyLevel)
	return true, reqAccuracyLevel, nil
}

func (r *redshiftRunner) registerGeoClueAgent() error {
	if r.geoAgentRegistered {
		return nil
	}
	if r.sysService == nil {
		return errors.New("sys service is nil")
	}

	sysBus := r.sysService.Conn()
	agent := &geoClueAgent{
		MaxAccuracyLevel: AccuracyLevelStreet,
	}
	err := r.sysService.Export(dbusPathGeoClueAgent, agent)
	if err != nil {
		return err
	}

	geoManager := geoclue2.NewManager(sysBus)
	err = geoManager.AddAgent(0, "geoclue-demo-agent")
	if err != nil {
		return err
	}

	r.geoAgentRegistered = true
	return nil
}
