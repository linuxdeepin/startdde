// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

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
	"syscall"
	"time"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/dde-api/soundutils"
	daemon "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.daemon"
	powermanager "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.powermanager"
	sysbt "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.bluetooth"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.dbus"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.systemd1"
	xeventmonitor "github.com/linuxdeepin/go-dbus-factory/com.deepin.api.xeventmonitor"
	gio "github.com/linuxdeepin/go-gir/gio-2.0"
	"github.com/linuxdeepin/go-lib/appinfo/desktopappinfo"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-lib/procfs"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/dpms"
	"github.com/linuxdeepin/startdde/autostop"
	"github.com/linuxdeepin/startdde/keyring"
	"github.com/linuxdeepin/startdde/watchdog"
	"github.com/linuxdeepin/startdde/wm_kwin"
	"github.com/linuxdeepin/startdde/xcursor"
	"github.com/linuxdeepin/startdde/xsettings"
)

const (
	sectionTheme     = "Theme"
	keyIconThemeName = "IconThemeName"
	keyFont          = "Font"
	keyMonoFont      = "MonFont"
	keyFontSize      = "FontSize"

	xsKeyQtFontName     = "Qt/FontName"
	xsKeyQtMonoFontName = "Qt/MonoFontName"

	ddeLockDesktopFile = "/usr/share/applications/dde-lock.desktop"
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
	loginSession          login1.Session
	dbusDaemon            ofdbus.DBus          // session bus daemon
	sigLoop               *dbusutil.SignalLoop // session bus signal loop
	inhibitManager        InhibitManager
	powerManager          powermanager.PowerManager
	sysBt                 sysbt.Bluetooth

	CurrentSessionPath  dbus.ObjectPath
	objLogin            login1.Manager
	objLoginSessionSelf login1.Session
	daemon              daemon.Daemon
	inCallRequestLock   bool
	inCallRequestLockMu sync.Mutex

	//nolint
	signals *struct {
		Unlock                           struct{}
		InhibitorAdded, InhibitorRemoved struct {
			path dbus.ObjectPath
		}
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

const (
	dpmsStateOn int32 = iota
	dpmsStateStandBy
	dpmsStateSuspend
	dpmsStateOff
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

func stopRedshift() {
	bus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return
	}
	systemdManger := systemd1.NewManager(bus)
	_, err = systemdManger.StopUnit(0, "redshift.service", "replace")
	if err != nil {
		logger.Warning("failed to stop redshift.service:", err)
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
	stopRedshift()
	quitObexService()
	//注销系统断开所有蓝牙连接
	m.sysBt.DisconnectAudioDevices(0)

	if !force && soundutils.CanPlayEvent(soundutils.EventDesktopLogout) {
		// playLogoutSound 内部会退出 pulseaudio
		playLogoutSound()
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
	if m.loginSession == nil {
		return
	}
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
	str, err := m.objLogin.CanPowerOff(0) // 当前能否关机
	if err != nil {
		logger.Warning(err)
		return false, dbusutil.ToError(err)
	}
	return str == "yes", nil
}

func (m *SessionManager) Shutdown() *dbus.Error {
	m.launch(cmdShutdown, false)
	return nil
}

func (m *SessionManager) prepareShutdown(force bool) {
	killSogouImeWatchdog()
	stopBAMFDaemon()
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
	if m.objLoginSessionSelf != nil {
		err = m.objLoginSessionSelf.Terminate(0)
		if err != nil {
			logger.Warning("failed to terminate session self:", err)
		}
	}
	os.Exit(0)
}

func (m *SessionManager) CanReboot() (bool, *dbus.Error) {
	str, err := m.objLogin.CanReboot(0) // 当前能否重启
	if err != nil {
		logger.Warning(err)
		return false, dbusutil.ToError(err)
	}
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
	m.clearCurrentTty()

	err := m.objLogin.Reboot(0, false)
	if err != nil {
		logger.Warning("failed to call login Reboot:", err)
	}

	if _gSettingsConfig.needQuickBlackScreen {
		setDPMSMode(false)
	}
	if m.objLoginSessionSelf != nil {
		err = m.objLoginSessionSelf.Terminate(0)
		if err != nil {
			logger.Warning("failed to terminate session self:", err)
		}
	}
	os.Exit(0)
}

func (m *SessionManager) CanSuspend() (bool, *dbus.Error) {
	if os.Getenv("POWER_CAN_SLEEP") == "0" {
		logger.Info("can not Suspend, env POWER_CAN_SLEEP == 0")
		return false, nil
	}
	can, err := m.powerManager.CanSuspend(0) // 是否支持待机
	if err != nil {
		logger.Warning(err)
		return false, dbusutil.ToError(err)
	}
	if can {
		str, err := m.objLogin.CanSuspend(0) // 当前能否待机
		if err != nil {
			logger.Warning(err)
			return false, dbusutil.ToError(err)
		}
		return str == "yes", nil
	}
	return false, nil
}

func (m *SessionManager) RequestSuspend() *dbus.Error {
	_, err := os.Stat("/etc/deepin/no_suspend")
	if err == nil {
		// no suspend
		time.Sleep(time.Second)
		setDPMSMode(false)
		return nil
	}

	// 使用窗管接口进行黑屏处理
	if _gSettingsConfig.needQuickBlackScreen {
		logger.Info("request wm blackscreen effect")
		if _useWayland {
			setDpmsModeByKwin(dpmsStateOff)
		} else {
			cmd := exec.Command("/bin/bash", "-c", "dbus-send --print-reply --dest=org.kde.KWin /BlackScreen org.kde.kwin.BlackScreen.setActive boolean:true")
			error := cmd.Run()
			if error != nil {
				logger.Warning("wm blackscreen failed")
			}
		}

	}

	logger.Info("login1 start suspend")
	err = m.objLogin.Suspend(0, false)
	if err != nil {
		logger.Warning("failed to suspend:", err)
	}

	return nil
}

func (m *SessionManager) CanHibernate() (bool, *dbus.Error) {
	if os.Getenv("POWER_CAN_SLEEP") == "0" {
		logger.Info("can not Hibernate, env POWER_CAN_SLEEP == 0")
		return false, nil
	}
	can, err := m.powerManager.CanHibernate(0) // 是否支持休眠
	if err != nil {
		logger.Warning(err)
		return false, dbusutil.ToError(err)
	}
	if can {
		str, err := m.objLogin.CanHibernate(0) // 当前能否休眠
		if err != nil {
			logger.Warning(err)
			return false, dbusutil.ToError(err)
		}
		return str == "yes", nil
	}
	return false, nil
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

	if exe == "/usr/bin/dde-lock" {
		m.setLocked(value)
		return nil
	}

	cmd, err := process.Cmdline()
	if err != nil {
		return dbusutil.ToError(err)
	}

	desktopFile := cmd[0]
	if desktopFile != ddeLockDesktopFile {
		return dbusutil.ToError(fmt.Errorf("exe %q is invalid", exe))
	}

	info, err := desktopappinfo.NewDesktopAppInfoFromFile(desktopFile)
	if err != nil {
		return dbusutil.ToError(fmt.Errorf("desktop file %q is invalid", desktopFile))
	}
	exe = info.GetExecutable()
	if exe != "/usr/bin/dde-lock" {
		return dbusutil.ToError(fmt.Errorf("exe %q of desktop file %q is invalid", exe, desktopFile))
	}

	m.setLocked(value)
	return nil
}

func (m *SessionManager) setLocked(value bool) {
	logger.Debug("call setLocked", value)
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
}

func (m *SessionManager) getLocked() bool {
	m.mu.Lock()
	v := m.Locked
	m.mu.Unlock()
	return v
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
	if m.objLoginSessionSelf != nil {
		m.objLoginSessionSelf.InitSignalExt(sysSigLoop, true)
		err = m.objLoginSessionSelf.Active().ConnectChanged(func(hasValue bool, active bool) {
			logger.Debugf("session status changed hasValue: %v, active: %v", hasValue, active)
			if !hasValue {
				return
			}

			if active {
				// 变活跃
			} else {
				// 变不活跃
				isPreparingForSleep, _ := m.objLogin.PreparingForSleep().Get(0)
				// 待机时不在这里锁屏
				if !isPreparingForSleep {
					logger.Debug("call objLoginSessionSelf.Lock")
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
	}
}

func newSessionManager(service *dbusutil.Service) *SessionManager {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("newSessionManager recover:", err)
			return
		}
	}()
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
	sysBt := sysbt.NewBluetooth(sysBus)
	var objLoginSessionSelf login1.Session
	sessionPath, err := objLogin.GetSessionByPID(0, 0)
	if err != nil {
		logger.Warning("failed to get current session path:", err)
	} else {
		logger.Info("session path:", sessionPath)
		objLoginSessionSelf, err = login1.NewSession(sysBus, sessionPath)
		if err != nil {
			logger.Warning("login1.NewSession err:", err)
		}
	}

	m := &SessionManager{
		service:             service,
		cookies:             make(map[string]chan time.Time),
		sigLoop:             sigLoop,
		objLogin:            objLogin,
		objLoginSessionSelf: objLoginSessionSelf,
		powerManager:        powerManager,
		sysBt:               sysBt,
		dbusDaemon:          dbusDaemon,
		daemon:              daemon.NewDaemon(sysBus),
	}

	// 此处将init的操作提前，避免SessionManager对象被创建了，相关属性值还没有被初始化
	// 如果其它进程读取了非法的属性值， dbus连接会被关闭，导致startdde启动失败
	m.initSession()
	m.init()
	go initXEventMonitor()

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
		// 要保证 CurrentSessionPath 属性值的合法性
		m.CurrentSessionPath = "/"
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

func (m *SessionManager) start(xConn *x.Conn, sysSignalLoop *dbusutil.SignalLoop, service *dbusutil.Service) *SessionManager {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("StartSession recover:", err)
			return
		}
	}()

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

func getLoginSession() (login1.Session, error) {
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
	go func() {
		// 在特殊情况下，比如用 dde-switchtogreeter 命令切换到 greeter, 即切换到其他 tty,
		// RequestLock 方法不能立即返回。

		// 如果已经锁定，则立即返回
		locked := m.getLocked()
		if locked {
			logger.Debug("handleLoginSessionLock locked is true, return")
			return
		}

		// 使用 m.inCallRequestLock 防止同时多次调用 RequestLock，以简化状况。
		m.inCallRequestLockMu.Lock()
		if m.inCallRequestLock {
			m.inCallRequestLockMu.Unlock()
			// 如果已经发出一个锁屏请求，则立即返回。
			logger.Debug("handleLoginSessionLock inCall is true, return")
			return
		}
		m.inCallRequestLock = true
		m.inCallRequestLockMu.Unlock()

		t0 := time.Now()
		logger.Debug("handleLoginSessionLock call RequestLock begin")
		err := m.RequestLock()
		elapsed := time.Since(t0)
		logger.Debugf("handleLoginSessionLock call RequestLock end, cost: %v", elapsed)
		if err != nil {
			logger.Warning("failed to request lock:", err)
		}

		m.inCallRequestLockMu.Lock()
		m.inCallRequestLock = false
		m.inCallRequestLockMu.Unlock()
	}()

}

func (m *SessionManager) handleLoginSessionUnlock() {
	if m.loginSession == nil {
		return
	}
	logger.Debug("login session unlock")
	isActive, err := m.loginSession.Active().Get(0)
	if err != nil {
		logger.Warning("failed to get session property Active:", err)
		return
	}

	if isActive {
		// 图形界面 session 活跃
		m.emitSignalUnlock()
	} else {
		// 图形界面 session 不活跃，此时采用 kill 掉锁屏前端的特殊操作，
		// 如果不 kill 掉它，则一旦 session 变活跃，锁屏前端会在很近的时间内执行锁屏和解锁，
		// 会使用系统通知报告锁屏失败。 而在此种特殊情况下 kill 掉它，并不会造成明显问题。
		err = m.killLockFront()
		if err != nil {
			logger.Warning("failed to kill lock front:", err)
			m.emitSignalUnlock()
		} else {
			m.setLocked(false)
		}
	}
}

// 发送 Unlock 信号，前端锁屏程序收到后会立即解锁。
func (m *SessionManager) emitSignalUnlock() {
	logger.Debug("emit signal Unlock")
	err := m.service.Emit(m, "Unlock")
	if err != nil {
		logger.Warning("failed to emit signal Unlock:", err)
	}
}

// kill 掉前端锁屏程序
func (m *SessionManager) killLockFront() error {
	sessionBus := m.service.Conn()
	dbusDaemon := ofdbus.NewDBus(sessionBus)
	owner, err := dbusDaemon.GetNameOwner(0, "com.deepin.dde.lockFront")
	if err != nil {
		return err
	}

	pid, err := m.service.GetConnPID(owner)
	if err != nil {
		return err
	}
	logger.Debugf("kill lock front owner: %v, pid: %v", owner, pid)

	p, err := os.FindProcess(int(pid))
	// linux 系统下这里不会失败
	if err != nil {
		return err
	}

	// 发送 SIGTERM 礼貌的退出信号
	err = p.Signal(syscall.SIGTERM)
	if err != nil {
		return err
	}

	return nil
}

func setDPMSMode(on bool) {
	var err error
	if _useWayland {
		if !on {
			setDpmsModeByKwin(dpmsStateOff)
		} else {
			setDpmsModeByKwin(dpmsStateOn)
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
	if m.objLoginSessionSelf != nil {
		err := m.objLoginSessionSelf.Terminate(0)
		if err != nil {
			logger.Warning("LoginSessionSelf Terminate failed:", err)
		}
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

func quitObexService() {
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

func setDpmsModeByKwin(mode int32) {
	logger.Debug("[startdde] Set DPMS State", mode)

	sessionBus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return
	}
	sessionObj := sessionBus.Object("com.deepin.daemon.KWayland", "/com/deepin/daemon/KWayland/DpmsManager")
	var ret []dbus.Variant
	err = sessionObj.Call("com.deepin.daemon.KWayland.DpmsManager.dpmsList", 0).Store(&ret)
	if err != nil {
		logger.Warning(err)
		return
	}

	for i := 0; i < len(ret); i++ {
		v := ret[i].Value().(string)
		sessionObj := sessionBus.Object("com.deepin.daemon.KWayland", dbus.ObjectPath(v))
		err = sessionObj.Call("com.deepin.daemon.KWayland.Dpms.setDpmsMode", 0, int32(mode)).Err
		if err != nil {
			logger.Warning(err)
			return
		}
	}

	return
}

func initXEventMonitor() {
	bus, _ := dbus.SessionBus()
	xEvent := xeventmonitor.NewXEventMonitor(bus)
	sigLoop := dbusutil.NewSignalLoop(bus, 1)
	sigLoop.Start()
	xEvent.InitSignalExt(sigLoop, true)
	xEvent.ConnectButtonPress(func(button int32, x int32, y int32, id string) {
		// 4表示鼠标中键上滑，5表示鼠标中键下滑
		if button == 4 || button == 5 {
			setDPMSMode(true)
		}
	})
}