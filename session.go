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
	//dShut    *sessionmanager.SessionManager
	dConsole *consolekit.Manager
	dPower   *upower.Upower
)

/*
func (m *SessionManager) CanLogout() bool {
	if IsInhibited(1) {
		return false
	}

	return true
}
*/

func (m *SessionManager) Logout() {
	ExecCommand(_SHUTDOWN_CMD, _LOGOUT_ARG)
}

func (m *SessionManager) RequestLogout() {
	//dShut.Logout(1)
	os.Exit(0)
}

func (m *SessionManager) ForceLogout() {
	//dShut.Logout(2)
	os.Exit(0)
}

/*
func (shudown *SessionManager) CanShutdown() bool {
	if IsInhibited(1) {
		return false
	}

	return true
}
*/

func (m *SessionManager) Shutdown() {
	ExecCommand(_SHUTDOWN_CMD, _SHUTDOWN_ARG)
}

/*
func (m *SessionManager) RequestShutdown() {
	dShut.RequestShutdown()
}
*/

func (m *SessionManager) ForceShutdown() {
	dConsole.Stop()
}

/*
func (shudown *SessionManager) CanReboot() bool {
	if IsInhibited(1) {
		return false
	}

	return true
}
*/

func (m *SessionManager) Reboot() {
	ExecCommand(_SHUTDOWN_CMD, _REBOOT_ARG)
}

/*
func (m *SessionManager) RequestReboot() {
	dShut.RequestReboot()
}
*/

func (m *SessionManager) ForceReboot() {
	dConsole.Restart()
}

/*
func (m *SessionManager) CanSuspend() bool {
	if IsInhibited(4) {
		return false
	}

	return true
}
*/

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
	ExecCommand(_LOCK_EXEC, "")
}

func (m *SessionManager) PowerOffChoose() {
	ExecCommand(_SHUTDOWN_CMD, _POWER_CHOOSE_ARG)
}

func ExecCommand(cmd string, arg string) {
	err := exec.Command(cmd, arg).Run()
	if err != nil {
		panic(fmt.Sprintf("Exec '%s %s' Failed: %s\n",
			cmd, arg, err))
	}
}

/*
func IsInhibited(action uint32) bool {
	ok, err := dShut.IsInhibited(action)
	if err != nil {
		fmt.Println("IsInhibited Failed:", err)
		return true
	}

	return ok
}
*/

func InitSession() {
	var err error

	/*
		dShut, err = sessionmanager.NewSessionManager("/org/gnome/SessionManager")
		if err != nil {
			fmt.Println("session: New SessionManager Failed:", err)
			return
		}
	*/

	dConsole, err = consolekit.NewManager("/org/freedesktop/ConsoleKit/Manager")
	if err != nil {
		panic(fmt.Sprintln("consolekit: New Manager Failed:", err))
	}

	dPower, err = upower.NewUpower("/org/freedesktop/UPower")
	if err != nil {
		panic(fmt.Sprintln("upower: New Upower Failed:", err))
	}
}

func StartSession() {
	InitSession()
	mShut := &SessionManager{}
	err := dbus.InstallOnSession(mShut)
	if err != nil {
		panic(fmt.Sprintln("Install Session DBus Failed:", err))
	}
}
