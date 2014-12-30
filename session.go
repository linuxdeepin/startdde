package main

import (
	screenlock "dbus/com/deepin/dde/screenlock/frontend"
	"dbus/org/freedesktop/login1"
	"fmt"
	"os"
	"pkg.linuxdeepin.com/lib/dbus"
	"time"
)

type SessionManager struct {
	CurrentUid string
	cookies    map[string]chan time.Time
	Stage      int32
}

const (
	_LOCK_EXEC    = "/usr/bin/dde-lock"
	_SHUTDOWN_CMD = "/usr/bin/dde-shutdown"
)

const (
	SessionStageInitBegin int32 = iota
	SessionStageInitEnd
	SessionStageCoreBegin
	SessionStageCoreEnd
	SessionStageAppsBegin
	SessionStageAppsEnd
)

var (
	objLogin *login1.Manager
)

func (m *SessionManager) CanLogout() bool {
	return true
}

func (m *SessionManager) Logout() {
	execCommand(_SHUTDOWN_CMD, "")
}

func (m *SessionManager) RequestLogout() {
	os.Exit(0)
}

func (m *SessionManager) ForceLogout() {
	os.Exit(0)
}

func (shudown *SessionManager) CanShutdown() bool {
	str, _ := objLogin.CanPowerOff()
	if str == "yes" {
		return true
	}

	return false
}

func (m *SessionManager) Shutdown() {
	execCommand(_SHUTDOWN_CMD, "")
}

func (m *SessionManager) RequestShutdown() {
	objLogin.PowerOff(true)
}

func (m *SessionManager) ForceShutdown() {
	objLogin.PowerOff(false)
}

func (shudown *SessionManager) CanReboot() bool {
	str, _ := objLogin.CanReboot()
	if str == "yes" {
		return true
	}

	return false
}

func (m *SessionManager) Reboot() {
	execCommand(_SHUTDOWN_CMD, "")
}

func (m *SessionManager) RequestReboot() {
	objLogin.Reboot(true)
}

func (m *SessionManager) ForceReboot() {
	objLogin.Reboot(false)
}

func (m *SessionManager) CanSuspend() bool {
	str, _ := objLogin.CanSuspend()
	if str == "yes" {
		return true
	}
	return false
}

func (m *SessionManager) RequestSuspend() {
	objLogin.Suspend(false)
}

func (m *SessionManager) CanHibernate() bool {
	str, _ := objLogin.CanHibernate()
	if str == "yes" {
		return true
	}
	return false
}

func (m *SessionManager) RequestHibernate() {
	objLogin.Hibernate(false)
}

func (m *SessionManager) RequestLock() error {
	guiOk := make(chan bool, 1)
	lock, err := screenlock.NewScreenlock("com.deepin.dde.screenlock.Frontend", "/com/deepin/dde/screenlock/Frontend")
	if err != nil {
		return err
	}
	defer screenlock.DestroyScreenlock(lock)
	lock.ConnectReady(func() {
		guiOk <- true
	})
	lock.Hello()
	select {
	case <-time.After(time.Second * 5):
		return fmt.Errorf("wait lock gui showing timeout")
	case <-guiOk:
		return nil
	}
}

func (m *SessionManager) PowerOffChoose() {
	execCommand(_SHUTDOWN_CMD, "")
}

func initSession() {
	var err error

	objLogin, err = login1.NewManager("org.freedesktop.login1",
		"/org/freedesktop/login1")
	if err != nil {
		panic(fmt.Errorf("New Login1 Failed: %s", err))
	}
}

func newSessionManager() *SessionManager {
	m := &SessionManager{}
	m.cookies = make(map[string]chan time.Time)
	m.setPropName("CurrentUid")

	return m
}
func (manager *SessionManager) launchWindowManager() {
	initSplash()
	switch *WindowManager {
	case "compiz":
		manager.launch("/usr/bin/gtk-window-decorator", false)
		manager.launch("/usr/bin/compiz", false)
	case "deepin":
		//TODO: need special handle? like notify SessionManager
		manager.launch("/usr/bin/deepin-wm", false)
	default:
		logger.Warning("the window manager of", *WindowManager, "may be not supported")
		manager.launch(*WindowManager, false)
	}
	initSplashAfterDependsLoaded()
}

func startSession() {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("StartSession recover:", err)
			return
		}
	}()

	initSession()
	manager := newSessionManager()
	err := dbus.InstallOnSession(manager)
	if err != nil {
		logger.Error("Install Session DBus Failed:", err)
		return
	}

	manager.setPropStage(SessionStageInitBegin)
	manager.launchWindowManager()
	manager.setPropStage(SessionStageInitEnd)

	manager.setPropStage(SessionStageCoreBegin)
	startStartManager()

	manager.launch("/usr/lib/deepin-daemon/dde-session-daemon", true)

	manager.ShowGuideOnce()
	manager.launch("/usr/bin/dde-launcher", false, "--hidden")

	manager.launch("/usr/bin/dde-desktop", true)
	manager.launch("/usr/bin/dde-dock", true)
	manager.launch("/usr/bin/dde-dock-applets", false)

	manager.setPropStage(SessionStageCoreEnd)

	manager.setPropStage(SessionStageAppsBegin)

	if !*debug {
		startAutostartProgram()
	}
	manager.setPropStage(SessionStageAppsEnd)
}

func (m *SessionManager) ShowGuideOnce() bool {
	path := os.ExpandEnv("$HOME/.config/not_first_run_dde")
	_, err := os.Stat(path)
	if err != nil {
		f, err := os.Create(path)
		defer f.Close()
		if err != nil {
			logger.Error("Can't initlize first dde", err)
			return false
		}

		m.launch("/usr/lib/deepin-daemon/dde-guide", true)
		return true
	}

	return false
}
