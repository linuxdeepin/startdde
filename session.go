package main

import (
	"dbus/org/freedesktop/consolekit"
	"dbus/org/freedesktop/upower"
	//"dbus/org/gnome/sessionmanager"
	"dlib/dbus"
	"fmt"
	"os"
	"os/exec"
)

type SessionManager struct{}

const (
	_LOCK_EXEC        = "/usr/bin/dlock"
	_SHUTDOWN_CMD     = "/usr/lib/deepin-daemon/dshutdown"
	_REBOOT_ARG       = "--reboot"
	_LOGOUT_ARG       = "--logout"
	_SHUTDOWN_ARG     = "--shutdown"
	_POWER_CHOOSE_ARG = "--choice"
)

var (
	dConsole *consolekit.Manager
	dPower   *upower.Upower
)

func (m *SessionManager) CanLogout() bool {
	return true
}

func (m *SessionManager) Logout() {
	execCommand(_SHUTDOWN_CMD, _LOGOUT_ARG)
}

func (m *SessionManager) RequestLogout() {
	os.Exit(0)
}

func (m *SessionManager) ForceLogout() {
	os.Exit(0)
}

func (shudown *SessionManager) CanShutdown() bool {
	return true
}

func (m *SessionManager) Shutdown() {
	execCommand(_SHUTDOWN_CMD, _SHUTDOWN_ARG)
}

func (m *SessionManager) RequestShutdown() {
	m.ForceShutdown()
}

func (m *SessionManager) ForceShutdown() {
	dConsole.Stop()
}

func (shudown *SessionManager) CanReboot() bool {
	return true
}

func (m *SessionManager) Reboot() {
	execCommand(_SHUTDOWN_CMD, _REBOOT_ARG)
}

func (m *SessionManager) RequestReboot() {
	m.ForceReboot()
}

func (m *SessionManager) ForceReboot() {
	dConsole.Restart()
}

func (m *SessionManager) CanSuspend() bool {
	return true
}

func (m *SessionManager) RequestSuspend() {
	dPower.Suspend()
}

func (m *SessionManager) CanHibernate() bool {
	ok, _ := dPower.HibernateAllowed()
	return ok
}

func (m *SessionManager) RequestHibernate() {
	dPower.Hibernate()
}

func (m *SessionManager) RequestLock() {
	execCommand(_LOCK_EXEC, "")
}

func (m *SessionManager) PowerOffChoose() {
	execCommand(_SHUTDOWN_CMD, _POWER_CHOOSE_ARG)
}

func execCommand(cmd string, arg string) {
	err := exec.Command(cmd, arg).Run()
	if err != nil {
		fmt.Printf("Exec '%s %s' Failed: %s\n",
			cmd, arg, err)
	}
}

func initSession() {
	var err error

	dConsole, err = consolekit.NewManager("org.freedesktop.ConsoleKit", "/org/freedesktop/ConsoleKit/Manager")
	if err != nil {
		panic(fmt.Sprintln("consolekit: New Manager Failed:", err))
	}

	dPower, err = upower.NewUpower("org.freedesktop.UPower", "/org/freedesktop/UPower")
	if err != nil {
		panic(fmt.Sprintln("upower: New Upower Failed:", err))
	}
}

func startSession() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("StartSession recover: %s\n", err)
			return
		}
	}()

	initSession()
	mShut := &SessionManager{}
	err := dbus.InstallOnSession(mShut)
	if err != nil {
		fmt.Sprintln("Install Session DBus Failed:", err)
		return
	}
}
