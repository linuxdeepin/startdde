package main

import (
	"dbus/com/deepin/daemon/soundeffect"
	"dbus/org/freedesktop/login1"
	"fmt"
	"os"
	"pkg.deepin.io/lib/dbus"
	"sync"
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
	err := playSystemSound("logout", true)
	if err != nil {
		logger.Warning("Play 'logout' sound failed:", err)
	}
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
	err := playSystemSound("shutdown", true)
	if err != nil {
		logger.Warning("Play 'logout' sound failed:", err)
	}
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
	execCommand(_LOCK_EXEC, "")
	return nil
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
	manager.launch(*windowManagerBin, false)
}

func startSession() {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("StartSession recover:", err)
			return
		}
	}()

	initSession()

	// Fixed: Set `GNOME_DESKTOP_SESSION_ID` to cheat `xdg-open`
	// https://tower.im/projects/8162ac3745044ca29f9f3d21beaeb93d/todos/d51f8f2a317740cca3af15384d34e79f/
	os.Setenv("GNOME_DESKTOP_SESSION_ID", "this-is-deprecated")

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

	// apply display settings, becase of window manager reset it.
	// if wm not start finished, the operation will not work.
	initDisplaySettings()

	// dde-launcher is fast enough now, there's no need to start it at the begnning
	// of every session.
	// manager.launch("/usr/bin/dde-launcher", false, "--hidden")

	var wg sync.WaitGroup
	wg.Add(2)

	// dde-desktop and dde-dock-trash-plugin reply on deepin-file-manager-backend
	// to run properly.
	go func() {
		manager.launch("/usr/lib/deepin-daemon/deepin-file-manager-backend", true)
		manager.launch("/usr/bin/dde-desktop", true)
		wg.Done()
	}()

	go func() {
		manager.launch("/usr/lib/deepin-daemon/dde-preload", true)
		manager.launch("/usr/bin/dde-dock", true)
		manager.launch("/usr/lib/deepin-daemon/dde-session-daemon", false)
		wg.Done()
	}()
	wg.Wait()

	err = playSystemSound("login", false)
	if err != nil {
		logger.Warning("Play 'login' sound failed:", err)
	}

	manager.setPropStage(SessionStageCoreEnd)

	manager.setPropStage(SessionStageAppsBegin)

	if !*debug {
		startAutostartProgram()
	}
	manager.setPropStage(SessionStageAppsEnd)
}

func playSystemSound(event string, sync bool) error {
	player, err := soundeffect.NewSoundEffect("com.deepin.daemon.SoundEffect",
		"/com/deepin/daemon/SoundEffect")
	if err != nil {
		return err
	}
	defer soundeffect.DestroySoundEffect(player)

	if sync {
		return player.PlaySystemSoundSync(event)
	}

	return player.PlaySystemSound(event)
}
