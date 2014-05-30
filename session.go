package main

import (
	"dbus/org/freedesktop/login1"
	"dlib/dbus"
	"fmt"
	"os"
)

type SessionManager struct {
	CurrentUid string
}

const (
	_LOCK_EXEC        = "/usr/bin/dde-lock"
	_SHUTDOWN_CMD     = "/usr/bin/dde-shutdown"
	_REBOOT_ARG       = "--reboot"
	_LOGOUT_ARG       = "--logout"
	_SHUTDOWN_ARG     = "--shutdown"
	_POWER_CHOOSE_ARG = "--choice"
)

var (
	objLogin *login1.Manager
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
	str, _ := objLogin.CanPowerOff()
	if str == "yes" {
		return true
	}

	return false
}

func (m *SessionManager) Shutdown() {
	execCommand(_SHUTDOWN_CMD, _SHUTDOWN_ARG)
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
	execCommand(_SHUTDOWN_CMD, _REBOOT_ARG)
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

func (m *SessionManager) RequestLock() {
	execCommand(_LOCK_EXEC, "")
}

func (m *SessionManager) PowerOffChoose() {
	execCommand(_SHUTDOWN_CMD, _POWER_CHOOSE_ARG)
}

func initSession() {
	var err error

	objLogin, err = login1.NewManager("org.freedesktop.login1",
		"/org/freedesktop/login1")
	if err != nil {
		panic(fmt.Sprintln("New Login1 Failed: ", err))
	}
}

func newSessionManager() *SessionManager {
	m := &SessionManager{}
	m.setPropName("CurrentUid")

	return m
}

func startSession() {
	defer func() {
		if err := recover(); err != nil {
			Logger.Error("StartSession recover:", err)
			return
		}
	}()

	initSession()
	mShut := newSessionManager()
	err := dbus.InstallOnSession(mShut)
	if err != nil {
		Logger.Error("Install Session DBus Failed:", err)
		return
	}
}
