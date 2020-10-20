/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	dbus "github.com/godbus/dbus"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.dbus"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/dpms"
	"pkg.deepin.io/dde/api/soundutils"
	"pkg.deepin.io/dde/startdde/autostop"
	"pkg.deepin.io/dde/startdde/keyring"
	"pkg.deepin.io/dde/startdde/memchecker"
	"pkg.deepin.io/dde/startdde/swapsched"
	"pkg.deepin.io/dde/startdde/watchdog"
	"pkg.deepin.io/dde/startdde/wm"
	"pkg.deepin.io/dde/startdde/wm_kwin"
	"pkg.deepin.io/dde/startdde/xcursor"
	"pkg.deepin.io/dde/startdde/xsettings"
	gio "pkg.deepin.io/gir/gio-2.0"
	"pkg.deepin.io/lib/cgroup"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/keyfile"
	"pkg.deepin.io/lib/log"
	"pkg.deepin.io/lib/procfs"
	"pkg.deepin.io/lib/xdg/basedir"
)

type SessionManager struct {
	service               *dbusutil.Service
	mu                    sync.Mutex
	Locked                bool
	CurrentUid            string
	cookieLocker          sync.Mutex
	cookies               map[string]chan time.Time
	Stage                 int32
	allowSessionDaemonRun bool
	loginSession          *login1.Session
	dbusDaemon            *ofdbus.DBus // session dbus daemon
	sigLoop               *dbusutil.SignalLoop
	inhibitManager        InhibitManager

	CurrentSessionPath dbus.ObjectPath

	//nolint
	signals *struct {
		Unlock                           struct{}
		InhibitorAdded, InhibitorRemoved struct {
			path dbus.ObjectPath
		}
	}

	//nolint
	methods *struct {
		CanLogout             func() `out:"can"`
		CanShutdown           func() `out:"can"`
		CanReboot             func() `out:"can"`
		CanSuspend            func() `out:"can"`
		CanHibernate          func() `out:"can"`
		SetLocked             func() `in:"value"`
		AllowSessionDaemonRun func() `out:"allow"`
		Register              func() `in:"id" out:"ok"`
		Inhibit               func() `in:"appId,toplevelXid,reason,flags" out:"cookie"`
		IsInhibited           func() `in:"flags" out:"result"`
		Uninhibit             func() `in:"cookie"`
		GetInhibitors         func() `out:"inhibitors"`
	}
}

const (
	cmdShutdown      = "/usr/bin/dde-shutdown"
	lockFrontDest    = "com.deepin.dde.lockFront"
	lockFrontIfc     = lockFrontDest
	lockFrontObjPath = "/com/deepin/dde/lockFront"
)

const (
	SessionStageInitBegin int32 = iota
	SessionStageInitEnd
	SessionStageCoreBegin
	SessionStageCoreEnd // nolint
	SessionStageAppsBegin
	SessionStageAppsEnd
)

var (
	objLogin            *login1.Manager
	objLoginSessionSelf *login1.Session
	swapSchedDispatcher *swapsched.Dispatcher
)

func (m *SessionManager) CanLogout() (bool, *dbus.Error) {
	return true, nil
}

func (m *SessionManager) Logout() *dbus.Error {
	m.launch(cmdShutdown, false)
	return nil
}

func stopBAMFDaemon() {
	// NOTE: Proactively stop the bamfdaemon service.
	// If you don't do this, it will exit in the failed state because X Server exits,
	// causing the restart to be too frequent and not being started properly.
	// This is a temporary workaround.
	bus, err := dbus.SessionBus()
	if err == nil {
		systemdUser := bus.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")
		var jobPath dbus.ObjectPath
		err = systemdUser.Call("org.freedesktop.systemd1.Manager.StopUnit",
			dbus.FlagNoAutoStart, "bamfdaemon.service", "replace").Store(&jobPath)
		if err != nil {
			logger.Warning("failed to stop bamfdaemon.service:", err)
		}
	} else {
		logger.Warning(err)
	}
}

func (m *SessionManager) prepareLogout(force bool) {
	if !force {
		err := autostop.LaunchAutostopScripts(logger)
		if err != nil {
			logger.Warning("failed to run auto script:", err)
		}
	}

	killSogouImeWatchdog()
	// kill process LangSelector ,because LangSelector will not be kill by common logout
	killLangSelector()
	// quit at-spi-dbus-bus.service
	quitAtSpiService()
	stopBAMFDaemon()
	sendMsgToUserExperModule(UserLogoutMsg)
	quitObexSevice()
	if !force && soundutils.CanPlayEvent(soundutils.EventDesktopLogout) {
		playLogoutSound()
		// PulseAudio should have quit
	} else {
		quitPulseAudio()
	}
}

// kill process LangSelector by cmd "pkill -ef -u $UID /usr/lib/deepin-daemon/langselector"
func killLangSelector() {
	u, err := user.Current()
	if err != nil {
		logger.Debug("failed to get current user:", err)
	}
	out, err := exec.Command("pkill", "-ef", "-u", u.Uid, "/usr/lib/deepin-daemon/langselector").Output()
	if err != nil {
		logger.Debug("failed to kill langselector:", err)
	} else {
		logger.Infof("kill langselector out:%s", out)
	}
}

func (m *SessionManager) RequestLogout() *dbus.Error {
	logger.Info("RequestLogout")
	m.logout(false)
	return nil
}

func (m *SessionManager) ForceLogout() *dbus.Error {
	logger.Info("ForceLogout")
	m.logout(true)
	return nil
}

func (m *SessionManager) logout(force bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.prepareLogout(force)
	doLogout(force)
}

func (shudown *SessionManager) CanShutdown() (bool, *dbus.Error) {
	str, _ := objLogin.CanPowerOff(0)
	return str == "yes", nil
}

func (m *SessionManager) Shutdown() *dbus.Error {
	m.launch(cmdShutdown, false)
	return nil
}

func (m *SessionManager) prepareShutdown(force bool) {
	killSogouImeWatchdog()
	stopBAMFDaemon()
	sendMsgToUserExperModule(UserShutdownMsg)
	if !force {
		preparePlayShutdownSound()
	}
	quitPulseAudio()
}

func killSogouImeWatchdog() {
	out, err := exec.Command("pkill", "-ef", "sogouImeService").Output()
	if err != nil {
		logger.Debug("failed to kill sogouIme watchdog:", err)
	} else {
		logger.Infof("kill sogouIme out:%s", out)
	}
}

func (m *SessionManager) RequestShutdown() *dbus.Error {
	logger.Info("RequestShutdown")
	m.shutdown(false)
	return nil
}

func (m *SessionManager) ForceShutdown() *dbus.Error {
	logger.Info("ForceShutdown")
	m.shutdown(true)
	return nil
}

func (m *SessionManager) shutdown(force bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.prepareShutdown(force)
	err := objLogin.PowerOff(0, false)
	if err != nil {
		logger.Warning("failed to call login PowerOff:", err)
	}
	setDPMSMode(false)
	err = objLoginSessionSelf.Terminate(0)
	if err != nil {
		logger.Warning("failed to terminate session self:", err)
	}
	os.Exit(0)
}

func (shudown *SessionManager) CanReboot() (bool, *dbus.Error) {
	str, _ := objLogin.CanReboot(0)
	return str == "yes", nil
}

func (m *SessionManager) Reboot() *dbus.Error {
	m.launch(cmdShutdown, false)
	return nil
}

func (m *SessionManager) RequestReboot() *dbus.Error {
	logger.Info("RequestReboot")
	m.reboot(false)
	return nil
}

func (m *SessionManager) ForceReboot() *dbus.Error {
	logger.Info("ForceReboot")
	m.reboot(true)
	return nil
}

func (m *SessionManager) reboot(force bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.prepareShutdown(force)
	err := objLogin.Reboot(0, false)
	if err != nil {
		logger.Warning("failed to call login Reboot:", err)
	}
	setDPMSMode(false)
	err = objLoginSessionSelf.Terminate(0)
	if err != nil {
		logger.Warning("failed to terminate session self:", err)
	}
	os.Exit(0)
}

func (m *SessionManager) CanSuspend() (bool, *dbus.Error) {
	_, err := os.Stat("/sys/power/mem_sleep")
	if os.IsNotExist(err) {
		return false, nil
	}

	str, _ := objLogin.CanSuspend(0)
	return str == "yes", nil
}

func (m *SessionManager) RequestSuspend() *dbus.Error {
	_, err := os.Stat("/etc/deepin/no_suspend")
	if err == nil {
		// no suspend
		time.Sleep(time.Second)
		setDPMSMode(false)
		return nil
	}

	err = objLogin.Suspend(0, false)
	if err != nil {
		logger.Warning("failed to suspend:", err)
	}
	setDPMSMode(false)
	return nil
}

func (m *SessionManager) CanHibernate() (bool, *dbus.Error) {
	str, _ := objLogin.CanHibernate(0)
	return str == "yes", nil
}

func (m *SessionManager) RequestHibernate() *dbus.Error {
	err := objLogin.Hibernate(0, false)
	if err != nil {
		logger.Warning("failed to Hibernate:", err)
	}
	setDPMSMode(false)
	return nil
}

func (m *SessionManager) RequestLock() *dbus.Error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return dbusutil.ToError(err)
	}
	err = conn.Object(lockFrontDest, lockFrontObjPath).Call(lockFrontIfc+".Show", 0).Err
	return dbusutil.ToError(err)
}

func (m *SessionManager) PowerOffChoose() *dbus.Error {
	m.launch(cmdShutdown, false)
	return nil
}

func (m *SessionManager) ToggleDebug() *dbus.Error {
	if logger.GetLogLevel() == log.LevelDebug {
		doSetLogLevel(log.LevelInfo)
		logger.Debug("Debug mode disabled")
	} else {
		doSetLogLevel(log.LevelDebug)
		logger.Debug("Debug mode enabled")
	}
	return nil
}

func (m *SessionManager) SetLocked(sender dbus.Sender, value bool) *dbus.Error {
	pid, err := m.service.GetConnPID(string(sender))
	if err != nil {
		return dbusutil.ToError(err)
	}
	process := procfs.Process(pid)
	exe, err := process.Exe()
	if err != nil {
		return dbusutil.ToError(err)
	}

	if exe != "/usr/bin/dde-lock" {
		return dbusutil.ToError(fmt.Errorf("exe %q is invalid", exe))
	}

	m.setLocked(value)
	return nil
}

func (m *SessionManager) setLocked(value bool) {
	m.mu.Lock()
	if m.Locked != value {
		m.Locked = value
		err := m.service.EmitPropertyChanged(m, "Locked", value)
		if err != nil {
			logger.Warning(err)
		}
	}
	m.mu.Unlock()

	watchdogManager := watchdog.GetManager()
	if watchdogManager != nil {
		task := watchdogManager.GetTask("dde-lock")
		if task != nil {
			if value {
				if task.GetFailed() {
					task.Reset()
				}
			} else {
				task.Reset()
			}
		} else {
			logger.Warning("not found task dde-lock")
		}
	} else {
		logger.Warning("watchdogManager is nil")
	}

	if !value && m.loginSession != nil {
		// unlock
		err := m.loginSession.Unlock(0)
		if err != nil {
			logger.Warning("failed to unlock login session:", err)
		}
	}
}

func (m *SessionManager) getLocked() bool {
	m.mu.Lock()
	v := m.Locked
	m.mu.Unlock()
	return v
}

func callSwapSchedHelperPrepare(sessionID string) error {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	const dest = "com.deepin.daemon.SwapSchedHelper"
	obj := sysBus.Object(dest, "/com/deepin/daemon/SwapSchedHelper")
	return obj.Call(dest+".Prepare", 0, sessionID).Store()
}

func initSession() {
	var err error

	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		return
	}

	objLogin = login1.NewManager(sysBus)
	sessionPath, err := objLogin.GetSessionByPID(0, 0)
	if err != nil {
		panic(fmt.Errorf("get session path failed: %s", err))
	}

	objLoginSessionSelf, err = login1.NewSession(sysBus, sessionPath)
	if err != nil {
		panic(fmt.Errorf("new Login1 session failed: %s", err))
	}

	sysSigLoop := dbusutil.NewSignalLoop(sysBus, 10)
	sysSigLoop.Start()
	objLoginSessionSelf.InitSignalExt(sysSigLoop, true)
	err = objLoginSessionSelf.Active().ConnectChanged(func(hasValue bool, active bool) {
		logger.Debug("session status changed:", hasValue, active)
		if hasValue && !active {
			isPreparingForSleep, _ := objLogin.PreparingForSleep().Get(0)
			if !isPreparingForSleep {
				err = objLoginSessionSelf.Lock(0)
				if err != nil {
					logger.Warning("failed to Lock current session:", err)
				}
			}
		}
	})
	if err != nil {
		logger.Warning("failed to connect Active changed:", err)
	}
	if globalGSettingsConfig.swapSchedEnabled {
		initSwapSched()
	} else {
		logger.Info("swap sched disabled")
	}
}

func initSwapSched() {
	err := cgroup.Init()
	if err != nil {
		logger.Warning(err)
		return
	}

	globalCgExecBin, err = exec.LookPath("cgexec")
	if err != nil {
		logger.Warning("cgexec not found:", err)
		return
	}

	sessionID, err := objLoginSessionSelf.Id().Get(0)
	if err != nil {
		logger.Warning(err)
	}

	err = callSwapSchedHelperPrepare(sessionID)
	if err != nil {
		logger.Warning("call SwapSchedHelper.Prepare error:", err)
	}

	swapsched.SetLogger(logger)

	memCheckerCfg := memchecker.GetConfig()

	logger.Debugf("mem checker cfg min mem avail: %d KB", memCheckerCfg.MinMemAvail)
	var enableMemAvailMax int64 // unit is byte
	enableMemAvailMax = int64(memCheckerCfg.MinMemAvail*1024) - 100*swapsched.MB
	if enableMemAvailMax < 0 {
		enableMemAvailMax = 100 * swapsched.MB
	}

	swapSchedCfg := swapsched.Config{
		UIAppsCGroup:       sessionID + "@dde/uiapps",
		DECGroup:           sessionID + "@dde/DE",
		EnableMemAvailMax:  uint64(enableMemAvailMax),
		DisableMemAvailMin: uint64(enableMemAvailMax) + 200*swapsched.MB,
	}
	swapSchedDispatcher, err = swapsched.NewDispatcher(swapSchedCfg)
	logger.Debugf("swap sched config: %+v", swapSchedCfg)

	if err == nil {
		// add self to DE cgroup
		deCg := cgroup.NewCgroup(swapSchedDispatcher.GetDECGroup())
		deCg.AddController(cgroup.Memory)
		err = deCg.AttachCurrentProcess()
		if err != nil {
			logger.Warning("failed to add self to DE cgroup:", err)
		}

		go func() {
			err = swapsched.ActiveWindowHandler(swapSchedDispatcher.ActiveWindowHandler).Monitor()
			if err != nil {
				logger.Warning(err)
			}
		}()
		go swapSchedDispatcher.Balance()
	} else {
		logger.Warning("failed to new swap sched dispatcher:", err)
	}
}

func newSessionManager(service *dbusutil.Service) *SessionManager {
	m := &SessionManager{
		service: service,
	}
	m.cookies = make(map[string]chan time.Time)
	m.setPropName("CurrentUid")
	var err error
	m.loginSession, err = getLoginSession()
	if err != nil {
		logger.Warning("failed to get current login session:", err)
	}

	sessionBus := service.Conn()
	m.sigLoop = dbusutil.NewSignalLoop(sessionBus, 10)
	m.sigLoop.Start()
	m.dbusDaemon = ofdbus.NewDBus(sessionBus)
	m.CurrentSessionPath, err = getCurSessionPath()
	if err != nil {
		logger.Warning("failed to get current session path:", err)
	}

	m.initInhibitManager()
	m.listenDBusSignals()
	return m
}

func (manager *SessionManager) listenDBusSignals() {
	manager.dbusDaemon.InitSignalExt(manager.sigLoop, true)
	_, err := manager.dbusDaemon.ConnectNameOwnerChanged(func(name string, oldOwner string, newOwner string) {
		if newOwner == "" && oldOwner != "" && name == oldOwner &&
			strings.HasPrefix(name, ":") {
			// uniq name lost
			ih := manager.inhibitManager.handleNameLost(name)
			if ih != nil {
				err := manager.service.StopExport(ih)
				if err != nil {
					logger.Warning(err)
					return
				}

				err = manager.service.Emit(manager, signalInhibitorRemoved, ih.getPath())
				if err != nil {
					logger.Warning(err)
				}
			}
		}
	})
	if err != nil {
		logger.Warning(err)
	}
}

func (manager *SessionManager) launchWindowManager(wait bool) {
	if globalUseWayland {
		return
	}
	manager.setPropStage(SessionStageInitBegin)
	var useKwin bool = shouldUseDDEKwin()

	logger.Debug("Will launch wm")
	if useKwin {
		err := wm_kwin.Start(logger, globalWmChooserLaunched)
		if err != nil {
			logger.Warning(err)
		}
		manager.launch("kwin_no_scale", wait)
		return
	}

	err := wm.Start(logger, globalWmChooserLaunched, manager.service)
	if err != nil {
		logger.Error("Failed to start wm module:", err)
		return
	}
	manager.launch("env", wm.ShouldWait(), "GDK_SCALE=1", wm.GetWM())
	manager.setPropStage(SessionStageInitEnd)
}

func (m *SessionManager) launchDDE() {
	versionChanged, err := isDeepinVersionChanged()
	if err != nil {
		logger.Warning("failed to get deepin version changed:", err)
	}
	if versionChanged {
		err := showDDEWelcome()
		if err != nil {
			logger.Warning("failed to start dde-welcome:", err)
		}
	}

	osdRunning, err := isOSDRunning()
	if err != nil {
		logger.Warning(err)
	} else {
		if osdRunning {
			if globalXSManager.NeedRestartOSD() {
				logger.Info("Restart dde-osd")
				m.launch("/usr/lib/deepin-daemon/dde-osd", false)
			}
		} else {
			notificationsOwned, err := isNotificationsOwned()
			if err != nil {
				logger.Warning("failed to get org.freedesktop.Notifications status:", err)
			} else if !notificationsOwned {
				m.launch("/usr/lib/deepin-daemon/dde-osd", false)
			}
		}
	}

	groups, err := loadGroupFile()
	if err != nil {
		logger.Error("Failed to load launch group file:", err)
		return
	}

	sort.Sort(groups)

	for idx, group := range groups {
		logger.Debugf("[%d] group p%d start", idx, group.Priority)
		noWaitCount := 0
		for _, cmd := range group.Group {
			if cmd.Command == "wm" {
				logger.Debug(cmd.Command, cmd.Args, cmd.Wait)
				m.launchWindowManager(cmd.Wait)
				continue
			}

			if cmd.Wait {
				logger.Debug(cmd.Command, cmd.Args, cmd.Wait)
				m.launch(cmd.Command, true, cmd.Args...)
				continue
			}

			// no wait
			if noWaitCount == 0 {
				logger.Debug(cmd.Command, cmd.Args, cmd.Wait)
				m.launch(cmd.Command, false, cmd.Args...)

			} else {
				// noWaitCount > 0
				logger.Debug(cmd.Command, cmd.Args, cmd.Wait, "launch after",
					100*noWaitCount, "ms")

				closureCmd := struct {
					bin  string
					args []string
				}{
					cmd.Command,
					cmd.Args,
				}
				time.AfterFunc(100*time.Duration(noWaitCount)*time.Millisecond, func() {
					m.launch(closureCmd.bin, false, closureCmd.args...)
				})
			}

			noWaitCount++
		}
		logger.Debugf("[%d] group p%d end", idx, group.Priority)
	}
}

func (m *SessionManager) launchAutostart() {
	m.setPropStage(SessionStageAppsBegin)
	if !*debug {
		delay := globalGSettingsConfig.autoStartDelay
		logger.Debug("Autostart delay seconds:", delay)
		if delay > 0 {
			time.AfterFunc(time.Second*time.Duration(delay), func() {
				startAutostartProgram()
			})
		} else {
			startAutostartProgram()
		}
	}
	m.setPropStage(SessionStageAppsEnd)
}

func setupEnvironments() {
	envVars := make(map[string]string)
	// man gnome-keyring-daemon:
	// The daemon will print out various environment variables which should be set
	// in the user's environment, in order to interact with the daemon.
	gnomeKeyringOutput, err := exec.Command("/usr/bin/gnome-keyring-daemon", "--start",
		"--components=secrets,pkcs11,ssh").Output()
	if err == nil {
		lines := bytes.Split(gnomeKeyringOutput, []byte{'\n'})
		for _, line := range lines {
			keyValuePair := bytes.SplitN(line, []byte{'='}, 2)
			if len(keyValuePair) != 2 {
				continue
			}

			key := string(keyValuePair[0])
			value := string(keyValuePair[1])
			envVars[key] = value
		}
	} else {
		logger.Warning("exec gnome-keyring-daemon err:", err)
	}

	// Fixed: Set `GNOME_DESKTOP_SESSION_ID` to cheat `xdg-open`
	envVars["GNOME_DESKTOP_SESSION_ID"] = "this-is-deprecated"
	envVars["XDG_CURRENT_DESKTOP"] = "Deepin"

	scaleFactor := 1.0
	if globalXSManager != nil {
		scaleFactor, _ = globalXSManager.GetScaleFactor()
	}
	envVars["QT_DBL_CLICK_DIST"] = strconv.Itoa(int(15 * scaleFactor))
	envVars["QT_LINUX_ACCESSIBILITY_ALWAYS_ON"] = "1"

	// set scale factor for deepin wine apps
	if scaleFactor != 1.0 {
		envVars[xsettings.EnvDeepinWineScale] = strconv.FormatFloat(
			scaleFactor, 'f', 2, 64)
	}

	systemctlArgs := make([]string, 0, len(envVars)+2)
	systemctlArgs = append(systemctlArgs, "--user", "set-environment")
	for key, value := range envVars {
		logger.Debugf("set env %s = %q", key, value)
		err = os.Setenv(key, value)
		if err != nil {
			logger.Warning(err)
		}
		systemctlArgs = append(systemctlArgs, key+"="+value)
	}

	for _, envName := range []string{
		"LANG",
		"LANGUAGE",
	} {
		envValue, ok := os.LookupEnv(envName)
		if ok {
			envVars[envName] = envValue
		}
	}

	sessionBus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return
	}

	err = sessionBus.BusObject().Call("org.freedesktop.DBus."+
		"UpdateActivationEnvironment", 0, envVars).Err
	if err != nil {
		logger.Warning(err)
	}

	err = exec.Command("systemctl", systemctlArgs...).Run()
	if err != nil {
		logger.Warning("failed to set env for systemd-user:", err)
	}
}

const (
	sectionTheme     = "Theme"
	keyIconThemeName = "IconThemeName"
	keyFont          = "Font"
	keyMonoFont      = "MonFont"
	keyFontSize      = "FontSize"

	xsKeyQtFontName     = "Qt/FontName"
	xsKeyQtMonoFontName = "Qt/MonoFontName"
)

func initQtThemeConfig() error {
	appearanceGs := gio.NewSettings("com.deepin.dde.appearance")
	var needSave bool

	var defaultFont, defaultMonoFont string
	loadDefaultFontCfg := func() {
		filename := "/usr/share/deepin-default-settings/fontconfig.json"
		defaultFontCfg, err := loadDefaultFontConfig(filename)
		if err != nil {
			logger.Warning("failed to load default font config:", err)
		}
		defaultFont, defaultMonoFont = defaultFontCfg.Get()
		if defaultFont == "" {
			defaultFont = "Noto Sans"
		}
		if defaultMonoFont == "" {
			defaultMonoFont = "Noto Mono"
		}
	}

	xsQtFontName, err := globalXSManager.GetStringInternal(xsKeyQtFontName)
	if err != nil {
		logger.Warning(err)
	} else if xsQtFontName == "" {
		loadDefaultFontCfg()
		err = globalXSManager.SetStringInternal(xsKeyQtFontName, defaultFont)
		if err != nil {
			logger.Warning(err)
		}
	}

	xsQtMonoFontName, err := globalXSManager.GetStringInternal(xsKeyQtMonoFontName)
	if err != nil {
		logger.Warning(err)
	} else if xsQtMonoFontName == "" {
		loadDefaultFontCfg()
		err = globalXSManager.SetStringInternal(xsKeyQtMonoFontName, defaultMonoFont)
		if err != nil {
			logger.Warning(err)
		}
	}

	kf := keyfile.NewKeyFile()
	qtThemeCfgFile := filepath.Join(basedir.GetUserConfigDir(), "deepin/qt-theme.ini")
	err = kf.LoadFromFile(qtThemeCfgFile)
	if err != nil && !os.IsNotExist(err) {
		logger.Warning("failed to load qt-theme.ini:", err)
	}
	iconTheme, _ := kf.GetString(sectionTheme, keyIconThemeName)
	if iconTheme == "" {
		iconTheme = appearanceGs.GetString("icon-theme")
		kf.SetString(sectionTheme, keyIconThemeName, iconTheme)
		needSave = true
	}

	fontSize, _ := kf.GetFloat64(sectionTheme, keyFontSize)
	if fontSize == 0 {
		fontSize = appearanceGs.GetDouble("font-size")
		kf.SetFloat64(sectionTheme, keyFontSize, fontSize)
		needSave = true
	}

	font, _ := kf.GetString(sectionTheme, keyFont)
	if font == "" {
		if defaultFont == "" {
			loadDefaultFontCfg()
		}
		kf.SetString(sectionTheme, keyFont, defaultFont)
		needSave = true
	}

	monoFont, _ := kf.GetString(sectionTheme, keyMonoFont)
	if monoFont == "" {
		if defaultMonoFont == "" {
			loadDefaultFontCfg()
		}
		kf.SetString(sectionTheme, keyMonoFont, defaultMonoFont)
		needSave = true
	}

	if needSave {
		err = os.MkdirAll(filepath.Dir(qtThemeCfgFile), 0755)
		if err != nil {
			return err
		}
		return kf.SaveToFile(qtThemeCfgFile)
	}
	return nil
}

const (
	UserExperServiceName = "com.deepin.userexperience.Daemon"
	UserExperPath        = "/com/deepin/userexperience/Daemon"
	UserLoginMsg         = "login"
	UserLogoutMsg        = "logout"
	UserShutdownMsg      = "shutdown"

	UserExperOpenApp  = "openapp"
	UserExperCloseApp = "closeapp"

	UserExperCLoseAppChanInitLen = 24
)

type UeMessageItem struct {
	Path, Name, Id string
}

func sendMsgToUserExperModule(msg string) {
	// send message to user experience module
	// first send will active the services
	bus, err := dbus.SystemBus()
	ch := make(chan struct{})
	if err == nil {
		go func() {
			userexp := bus.Object(UserExperServiceName, UserExperPath)
			err = userexp.Call(UserExperServiceName+".SendLogonData", 0, msg).Err
			if err != nil {
				logger.Warningf("failed to call %s.SendLogonData, %v", UserExperServiceName, err)
			} else {
				logger.Infof("send %s message to ue module", msg)
			}
			close(ch)
		}()
		select {
		case <-ch:
		case <-time.After(1 * time.Second):
			logger.Debug("sendMsgToUserExperModule timed out!")
		}
	} else {
		logger.Warning(err)
	}
}

func startSession(conn *x.Conn, sysSignalLoop *dbusutil.SignalLoop, service *dbusutil.Service) *SessionManager {

	defer func() {
		if err := recover(); err != nil {
			logger.Error("StartSession recover:", err)
			return
		}
	}()

	initSession()

	manager := newSessionManager(service)
	err := service.Export(sessionManagerPath, manager)
	if err != nil {
		logger.Warning("export session manager failed:", err)
		return nil
	}

	setupEnvironments()

	err = initQtThemeConfig()
	if err != nil {
		logger.Warning("failed to init qt-theme.ini", err)
	}

	manager.setPropStage(SessionStageCoreBegin)
	startStartManager(conn, service)

	err = service.RequestName(sessionManagerServiceName)
	if err != nil {
		logger.Warningf("request name %q failed: %v", sessionManagerServiceName, err)
	}

	manager.launchDDE()
	go func() {
		setLeftPtrCursor()
		err := keyring.CheckLogin()
		if err != nil {
			logger.Warning("Failed to init keyring:", err)
		}
	}()
	time.AfterFunc(3*time.Second, START_MANAGER.listenAutostartFileEvents)
	go manager.launchAutostart()
	sendMsgToUserExperModule(UserLoginMsg)

	if manager.loginSession != nil {
		manager.loginSession.InitSignalExt(sysSignalLoop, true)
		_, err = manager.loginSession.ConnectLock(manager.handleLoginSessionLock)
		if err != nil {
			logger.Warning("failed to connect signal Lock:", err)
		}

		_, err = manager.loginSession.ConnectUnlock(manager.handleLoginSessionUnlock)
		if err != nil {
			logger.Warning("failled to connect signal Unlock:", err)
		}
	}

	return manager
}

func isDeepinVersionChanged() (changed bool, err error) {
	kfDeepinVersion := keyfile.NewKeyFile()
	err = kfDeepinVersion.LoadFromFile("/etc/deepin-version")
	if err != nil {
		return
	}

	v0, err := kfDeepinVersion.GetString("Release", "Version")
	if err != nil {
		return
	}

	kfDDEWelcome := keyfile.NewKeyFile()
	ddeWelcomeFile := filepath.Join(basedir.GetUserConfigDir(), "deepin/dde-welcome.conf")

	saveConfig := func() {
		kfDDEWelcome.SetString("General", "Version", v0)
		err := os.MkdirAll(filepath.Dir(ddeWelcomeFile), 0755)
		if err != nil {
			logger.Warning(err)
			return
		}

		tmpFile := ddeWelcomeFile + ".tmp"
		err = kfDDEWelcome.SaveToFile(tmpFile)
		if err != nil {
			logger.Warning("failed to save dde-welcome.conf:", err)
			return
		}

		err = syncFile(tmpFile)
		if err != nil {
			logger.Warning("failed to sync temp file:", err)
			return
		}

		err = os.Rename(tmpFile, ddeWelcomeFile)
		if err != nil {
			logger.Warning(err)
		}
	}

	err = kfDDEWelcome.LoadFromFile(ddeWelcomeFile)
	if err != nil {
		if os.IsNotExist(err) {
			// new user first login
			saveConfig()
			return false, nil
		}
		return false, err
	}

	v1, _ := kfDDEWelcome.GetString("General", "Version")

	if v0 != v1 {
		if v1 != "" {
			changed = true
		}
		saveConfig()
	}
	return
}

func setLeftPtrCursor() {
	gs := gio.NewSettings("com.deepin.xsettings")
	defer gs.Unref()

	theme := gs.GetString("gtk-cursor-theme-name")
	size := gs.GetInt("gtk-cursor-theme-size")

	err := xcursor.LoadAndApply(theme, "left_ptr", int(size))
	if err != nil {
		logger.Warning(err)
	}
}

func getLoginSession() (*login1.Session, error) {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	loginManager := login1.NewManager(sysBus)
	sessionPath, err := loginManager.GetSession(0, "")
	if err != nil {
		return nil, err
	}
	session, err := login1.NewSession(sysBus, sessionPath)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (m *SessionManager) handleLoginSessionLock() {
	logger.Debug("login session lock")
	err := m.RequestLock()
	if err != nil {
		logger.Warning("failed to request lock:", err)
	}
}

func (m *SessionManager) handleLoginSessionUnlock() {
	logger.Debug("login session unlock")
	err := m.service.Emit(m, "Unlock")
	if err != nil {
		logger.Warning("failed to emit signal Unlock:", err)
	}
}

func setDPMSMode(on bool) {
	var err error
	if globalUseWayland {
		if !on {
			_, err = exec.Command("dde_wldpms", "-s", "Off").Output()
		} else {
			_, err = exec.Command("dde_wldpms", "-s", "On").Output()
		}
	} else {
		var mode = uint16(dpms.DPMSModeOn)
		if !on {
			mode = uint16(dpms.DPMSModeOff)
		}
		err = dpms.ForceLevelChecked(XConn, mode).Check(XConn)
	}

	if err != nil {
		logger.Warning("Failed to set dpms mode:", on, err)
	}
}

func doLogout(force bool) {
	err := objLoginSessionSelf.Terminate(0)
	if err != nil {
		logger.Warning("LoginSessionSelf Terminate failed:", err)
	}
	os.Exit(0)
}

const (
	atSpiService = "at-spi-dbus-bus.service"
	obexService  = "obex.service"
)

func startAtSpiService() {
	time.Sleep(3 * time.Second)
	logger.Debugf("starting %s...", atSpiService)
	err := exec.Command("systemctl", "--user", "--runtime", "unmask", atSpiService).Run()
	if err != nil {
		logger.Warningf("unmask %s failed, err: %v", atSpiService, err)
		return
	}
	err = exec.Command("systemctl", "--user", "--runtime", "start", atSpiService).Run()
	if err != nil {
		logger.Warningf("start %s failed, err: %v", atSpiService, err)
		return
	}
}

func startObexService() {
	time.Sleep(3 * time.Second)
	logger.Debugf("starting %s...", obexService)
	err := exec.Command("systemctl", "--user", "--runtime", "unmask", obexService).Run()
	if err != nil {
		logger.Warningf("unmask %s failed, err: %v", obexService, err)
		return
	}
	err = exec.Command("systemctl", "--user", "--runtime", "start", obexService).Run()
	if err != nil {
		logger.Warningf("start %s failed, err: %v", obexService, err)
		return
	}
}

func quitAtSpiService() {
	logger.Debugf("quitting %s...", atSpiService)
	// mask at-spi-dbus-bus.service
	out, err := exec.Command("systemctl", "--user", "--runtime", "--now", "mask",
		atSpiService).CombinedOutput()
	if err != nil {
		logger.Warningf("temp mask %s failed err: %v, out: %s", atSpiService, err, out)
	}
	// view status
	err = exec.Command("systemctl", "--quiet", "--user", "is-active",
		atSpiService).Run()
	if err == nil {
		logger.Warningf("%s is still running", atSpiService)
	} else {
		logger.Debugf("%s is stopped, err: %v", atSpiService, err)
	}
}

func quitObexSevice() {
	logger.Debugf("quitting %s...", obexService)
	// mask obex.service
	out, err := exec.Command("systemctl", "--user", "--runtime", "--now", "mask",
		obexService).CombinedOutput()
	if err != nil {
		logger.Warningf("temp mask %s failed err: %v, out: %s", obexService, err, out)
	}
	// view status
	err = exec.Command("systemctl", "--quiet", "--user", "is-active",
		obexService).Run()
	if err == nil {
		logger.Warningf("%s is still running", obexService)
	} else {
		logger.Debugf("%s is stopped, err: %v", obexService, err)
	}
}

func getCurSessionPath() (dbus.ObjectPath, error) {
	var err error

	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		return "", err
	}

	loginManager := login1.NewManager(sysBus)
	sessionPath, err := loginManager.GetSessionByPID(0, 0)
	if err != nil {
		logger.Warning(err)
		return "", err
	}
	return sessionPath, nil
}
