// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-api/soundutils"
	sysbt "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.bluetooth1"
	daemon "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.daemon1"
	powermanager "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.powermanager1"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.dbus"
	login1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.login1"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.systemd1"
	"github.com/linuxdeepin/go-lib/appinfo/desktopappinfo"
	"github.com/linuxdeepin/go-lib/cgroup"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-lib/procfs"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/dpms"
	"github.com/linuxdeepin/startdde/memchecker"
	"github.com/linuxdeepin/startdde/swapsched"
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
	lockFrontDest    = "org.deepin.dde.LockFront1"
	lockFrontIfc     = lockFrontDest
	lockFrontObjPath = "/org/deepin/dde/LockFront1"
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
	// if !force {
	// 	err := autostop.LaunchAutostopScripts(logger)
	// 	if err != nil {
	// 		logger.Warning("failed to run auto script:", err)
	// 	}
	// }

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
		cmd := exec.Command("/bin/bash", "-c", "dbus-send --print-reply --dest=org.kde.KWin /BlackScreen org.kde.kwin.BlackScreen.setActive boolean:true")
		error := cmd.Run()
		if error != nil {
			logger.Warning("wm blackscreen failed")
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
	const dest = "org.deepin.dde.SwapSchedHelper1"
	obj := sysBus.Object(dest, "/org/deepin/dde/SwapSchedHelper1")
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

	if m.objLoginSessionSelf == nil {
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
	sessionPath, err := getCurSessionPath()
	if err != nil {
		logger.Warning("failed to get current session path:", err)
	} else {
		logger.Info("session path:", sessionPath)
		objLoginSessionSelf, err = login1.NewSession(sysBus, dbus.ObjectPath(sessionPath))
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
}

func (m *SessionManager) launchDDE() {
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

func (m *SessionManager) start(xConn *x.Conn, sysSignalLoop *dbusutil.SignalLoop, service *dbusutil.Service) *SessionManager {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("StartSession recover:", err)
			return
		}
	}()

	m.setPropStage(SessionStageCoreBegin)
	startStartManager(xConn, service)

	m.launchDDE()

	time.AfterFunc(3*time.Second, _startManager.listenAutostartFileEvents)

	if m.loginSession != nil {
		m.loginSession.InitSignalExt(sysSignalLoop, true)
		_, err := m.loginSession.ConnectLock(m.handleLoginSessionLock)
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

func getLoginSession() (login1.Session, error) {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	sessionPath, err := getCurSessionPath()
	if err != nil {
		return nil, err
	}

	session, err := login1.NewSession(sysBus, dbus.ObjectPath(sessionPath))
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
	owner, err := dbusDaemon.GetNameOwner(0, "org.deepin.dde.LockFront1")
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
	userPath, err := loginManager.GetUser(0, uint32(os.Getuid()))
	if err != nil {
		return "", err
	}

	userDBus, err := login1.NewUser(sysBus, userPath)
	if err != nil {
		logger.Warningf("new user failed: %v", err)
		return "", err
	}
	sessionPath, err := userDBus.Display().Get(0)
	if err != nil {
		logger.Warningf("fail to get display session info: %v", err)
		return "", err
	}
	return sessionPath.Path, nil
}
