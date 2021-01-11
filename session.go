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

	powermanager "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.powermanager"

	dbus "github.com/godbus/dbus"
	"github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.daemon"
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

const (
	sectionTheme     = "Theme"
	keyIconThemeName = "IconThemeName"
	keyFont          = "Font"
	keyMonoFont      = "MonFont"
	keyFontSize      = "FontSize"

	xsKeyQtFontName     = "Qt/FontName"
	xsKeyQtMonoFontName = "Qt/MonoFontName"
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
	dbusDaemon            *ofdbus.DBus         // session bus daemon
	sigLoop               *dbusutil.SignalLoop // session bus signal loop
	inhibitManager        InhibitManager
	powerManager          *powermanager.PowerManager

	CurrentSessionPath  dbus.ObjectPath
	objLogin            *login1.Manager
	objLoginSessionSelf *login1.Session
	daemon              *daemon.Daemon

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

func (m *SessionManager) clearCurrentTty() {
	vTNr, err := m.loginSession.VTNr().Get(0)
	if err != nil {
		logger.Warning("clearCurrentTty:", err)
		return
	}
	err = m.daemon.ClearTty(0, vTNr)
	if err != nil {
		logger.Warning("clearCurrentTty:", err)
	}
	return
}

func (m *SessionManager) logout(force bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.prepareLogout(force)
	m.clearCurrentTty()
	m.doLogout(force)
}

func (m *SessionManager) CanShutdown() (bool, *dbus.Error) {
	can, err := m.powerManager.CanShutdown(0)
	return can, dbusutil.ToError(err)
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
	m.clearCurrentTty()

	err := m.objLogin.PowerOff(0, false)
	if err != nil {
		logger.Warning("failed to call login PowerOff:", err)
	}

	if _gSettingsConfig.needQuickBlackScreen {
		setDPMSMode(false)
	}
	err = m.objLoginSessionSelf.Terminate(0)
	if err != nil {
		logger.Warning("failed to terminate session self:", err)
	}
	os.Exit(0)
}

func (m *SessionManager) CanReboot() (bool, *dbus.Error) {
	can, err := m.powerManager.CanReboot(0)
	return can, dbusutil.ToError(err)
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
	m.clearCurrentTty()

	err := m.objLogin.Reboot(0, false)
	if err != nil {
		logger.Warning("failed to call login Reboot:", err)
	}

	if _gSettingsConfig.needQuickBlackScreen {
		setDPMSMode(false)
	}
	err = m.objLoginSessionSelf.Terminate(0)
	if err != nil {
		logger.Warning("failed to terminate session self:", err)
	}
	os.Exit(0)
}

func (m *SessionManager) CanSuspend() (bool, *dbus.Error) {
	can, err := m.powerManager.CanSuspend(0)
	return can, dbusutil.ToError(err)
}

func (m *SessionManager) RequestSuspend() *dbus.Error {
	_, err := os.Stat("/etc/deepin/no_suspend")
	if err == nil {
		// no suspend
		time.Sleep(time.Second)
		setDPMSMode(false)
		return nil
	}

	err = m.objLogin.Suspend(0, false)
	if err != nil {
		logger.Warning("failed to suspend:", err)
	}

	if _gSettingsConfig.needQuickBlackScreen {
		setDPMSMode(false)
	}
	return nil
}

func (m *SessionManager) CanHibernate() (bool, *dbus.Error) {
	can, err := m.powerManager.CanHibernate(0)
	return can, dbusutil.ToError(err)
}

func (m *SessionManager) RequestHibernate() *dbus.Error {
	err := m.objLogin.Hibernate(0, false)
	if err != nil {
		logger.Warning("failed to Hibernate:", err)
	}
	if _gSettingsConfig.needQuickBlackScreen {
		setDPMSMode(false)
	}
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

func (m *SessionManager) initSession() {
	var err error

	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		return
	}

	sysSigLoop := dbusutil.NewSignalLoop(sysBus, 10)
	sysSigLoop.Start()
	m.objLoginSessionSelf.InitSignalExt(sysSigLoop, true)
	err = m.objLoginSessionSelf.Active().ConnectChanged(func(hasValue bool, active bool) {
		logger.Debug("session status changed:", hasValue, active)
		if hasValue && !active {
			isPreparingForSleep, _ := m.objLogin.PreparingForSleep().Get(0)
			if !isPreparingForSleep {
				err = m.objLoginSessionSelf.Lock(0)
				if err != nil {
					logger.Warning("failed to Lock current session:", err)
				}
			}
		}
	})
	if err != nil {
		logger.Warning("failed to connect Active changed:", err)
	}
	if _gSettingsConfig.swapSchedEnabled {
		m.initSwapSched()
	} else {
		logger.Info("swap sched disabled")
	}
}

func (m *SessionManager) initSwapSched() {
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

	sessionID, err := m.objLoginSessionSelf.Id().Get(0)
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
	var err error
	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		return nil
	}

	sessionBus := service.Conn()
	sigLoop := dbusutil.NewSignalLoop(sessionBus, 10)
	sigLoop.Start()
	dbusDaemon := ofdbus.NewDBus(sessionBus)
	dbusDaemon.InitSignalExt(sigLoop, true)
	objLogin := login1.NewManager(sysBus)
	powerManager := powermanager.NewPowerManager(sysBus)
	sessionPath, err := objLogin.GetSessionByPID(0, 0)
	if err != nil {
		panic(fmt.Errorf("get session path failed: %s", err))
	}
	objLoginSessionSelf, err := login1.NewSession(sysBus, sessionPath)
	if err != nil {
		panic(fmt.Errorf("new Login1 session failed: %s", err))
	}

	m := &SessionManager{
		service:             service,
		cookies:             make(map[string]chan time.Time),
		sigLoop:             sigLoop,
		objLogin:            objLogin,
		objLoginSessionSelf: objLoginSessionSelf,
		powerManager:        powerManager,
		dbusDaemon:          dbusDaemon,
		daemon:              daemon.NewDaemon(sysBus),
	}
	return m
}

func (m *SessionManager) init() {
	m.setPropName("CurrentUid")
	var err error
	m.loginSession, err = getLoginSession()
	if err != nil {
		logger.Warning("failed to get current login session:", err)
	}

	m.CurrentSessionPath, err = getCurSessionPath()
	if err != nil {
		logger.Warning("failed to get current session path:", err)
	}

	m.initInhibitManager()
	m.listenDBusSignals()
}

func (manager *SessionManager) listenDBusSignals() {
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

func (m *SessionManager) startWMSwitcher() {
	if _useWayland {
		return
	}
	if _useKWin {
		err := wm_kwin.Start(logger)
		if err != nil {
			logger.Warning(err)
		}
		return
	}
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

		var waitGroup sync.WaitGroup
		waitGroup.Add(len(group.Group))

		for _, cmd := range group.Group {
			cmd := cmd
			go func() {
				logger.Debug("run cmd:", cmd.Command, cmd.Args, cmd.Wait)
				m.launch(cmd.Command, cmd.Wait, cmd.Args...)
				waitGroup.Done()
			}()
		}

		waitGroup.Wait()

		logger.Debugf("[%d] group p%d end", idx, group.Priority)
	}
}

func (m *SessionManager) launchAutostart() {
	m.setPropStage(SessionStageAppsBegin)
	delay := _gSettingsConfig.autoStartDelay
	logger.Debug("autostart delay seconds:", delay)
	if delay > 0 {
		time.AfterFunc(time.Second*time.Duration(delay), func() {
			startAutostartProgram()
		})
	} else {
		startAutostartProgram()
	}
	m.setPropStage(SessionStageAppsEnd)
}

var _envVars = make(map[string]string, 17)

// 在启动核心组件之前设置一些环境变量
func setupEnvironments1() {
	// Fixed: Set `GNOME_DESKTOP_SESSION_ID` to cheat `xdg-open`
	_envVars["GNOME_DESKTOP_SESSION_ID"] = "this-is-deprecated"
	_envVars["XDG_CURRENT_DESKTOP"] = "Deepin"

	scaleFactor := xsettings.GetScaleFactor()
	_envVars["QT_DBL_CLICK_DIST"] = strconv.Itoa(int(15 * scaleFactor))
	_envVars["QT_LINUX_ACCESSIBILITY_ALWAYS_ON"] = "1"

	// set scale factor for deepin wine apps
	if scaleFactor != 1.0 {
		_envVars[xsettings.EnvDeepinWineScale] = strconv.FormatFloat(
			scaleFactor, 'f', 2, 64)
	}

	setEnvWithMap(_envVars)
}

func setEnvWithMap(envVars map[string]string) {
	for key, value := range envVars {
		logger.Debugf("set env %s = %q", key, value)
		err := os.Setenv(key, value)
		if err != nil {
			logger.Warning(err)
		}
	}
}

func setupEnvironments2() {
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
			_envVars[key] = value

			logger.Debugf("set env %s = %q", key, value)
			err := os.Setenv(key, value)
			if err != nil {
				logger.Warning(err)
			}
		}
	} else {
		logger.Warning("exec gnome-keyring-daemon err:", err)
	}

	for _, envName := range []string{
		"LANG",
		"LANGUAGE",
	} {
		envValue, ok := os.LookupEnv(envName)
		if ok {
			_envVars[envName] = envValue
		}
	}

	updateDBusEnv()
	updateSystemdUserEnv()
}

func updateDBusEnv() {
	sessionBus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return
	}

	err = sessionBus.BusObject().Call("org.freedesktop.DBus."+
		"UpdateActivationEnvironment", 0, _envVars).Err
	if err != nil {
		logger.Warning("update dbus env failed:", err)
	}
}

func updateSystemdUserEnv() {
	systemctlArgs := make([]string, 0, len(_envVars)+2)
	systemctlArgs = append(systemctlArgs, "--user", "set-environment")

	for key, value := range _envVars {
		systemctlArgs = append(systemctlArgs, key+"="+value)
	}

	err := exec.Command("systemctl", systemctlArgs...).Run()
	if err != nil {
		logger.Warning("failed to set env for systemd-user:", err)
	}
}

func initQtThemeConfig() error {
	appearanceGs := gio.NewSettings("com.deepin.dde.appearance")
	defer appearanceGs.Unref()
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

func (m *SessionManager) start(xConn *x.Conn, sysSignalLoop *dbusutil.SignalLoop, service *dbusutil.Service) *SessionManager {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("StartSession recover:", err)
			return
		}
	}()
	m.initSession()
	m.init()
	if _options.noXSessionScripts {
		runScript01DeepinProfileFaster()
		runScript30X11CommonXResourcesFaster()
		runScript90GpgAgentFaster()
	}
	setupEnvironments2()

	err := initQtThemeConfig()
	if err != nil {
		logger.Warning("failed to init qt-theme.ini", err)
	}
	m.setPropStage(SessionStageCoreBegin)
	startStartManager(xConn, service)

	m.startWMSwitcher()

	if _options.noXSessionScripts {
		startIMFcitx()
	}

	go startAtSpiService()
	// start obex.service
	go startObexService()
	m.launchDDE()

	go func() {
		setLeftPtrCursor()
		err := keyring.CheckLogin()
		if err != nil {
			logger.Warning("Failed to init keyring:", err)
		}
	}()
	time.AfterFunc(3*time.Second, _startManager.listenAutostartFileEvents)
	go m.launchAutostart()
	sendMsgToUserExperModule(UserLoginMsg)

	if m.loginSession != nil {
		m.loginSession.InitSignalExt(sysSignalLoop, true)
		_, err = m.loginSession.ConnectLock(m.handleLoginSessionLock)
		if err != nil {
			logger.Warning("failed to connect signal Lock:", err)
		}

		_, err = m.loginSession.ConnectUnlock(m.handleLoginSessionUnlock)
		if err != nil {
			logger.Warning("failled to connect signal Unlock:", err)
		}
	}

	return m
}

func startIMFcitx() {
	fcitxPath, err := exec.LookPath("fcitx")
	if err == nil {
		cmd := exec.Command(fcitxPath, "-d")
		go func() {
			err := cmd.Run()
			if err != nil {
				logger.Warning("fcitx exit with error:", err)
			}
		}()
	}
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
	if _useWayland {
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
		err = dpms.ForceLevelChecked(_xConn, mode).Check(_xConn)
	}

	if err != nil {
		logger.Warning("Failed to set dpms mode:", on, err)
	}
}

func (m *SessionManager) doLogout(force bool) {
	err := m.objLoginSessionSelf.Terminate(0)
	if err != nil {
		logger.Warning("LoginSessionSelf Terminate failed:", err)
	}
	os.Exit(0)
}

const (
	atSpiService = "at-spi-dbus-bus.service"
	obexService  = "obex.service"
)

// start at-spi-dbus-bus.service
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
