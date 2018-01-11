/*
 * Copyright (C) 2014 ~ 2017 Deepin Technology Co., Ltd.
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
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"dbus/org/freedesktop/login1"

	"pkg.deepin.io/dde/startdde/autostop"
	"pkg.deepin.io/dde/startdde/swapsched"

	"github.com/BurntSushi/xgbutil"
	"pkg.deepin.io/dde/api/soundutils"
	"pkg.deepin.io/dde/startdde/wm"
	"pkg.deepin.io/lib/cgroup"
	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/log"
)

type SessionManager struct {
	CurrentUid   string
	cookieLocker sync.Mutex
	cookies      map[string]chan time.Time
	Stage        int32
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
	SessionStageCoreEnd
	SessionStageAppsBegin
	SessionStageAppsEnd
)

var (
	objLogin            *login1.Manager
	objLoginSessionSelf *login1.Session
	swapSchedDispatcher *swapsched.Dispatcher
)

func (m *SessionManager) CanLogout() bool {
	return true
}

func (m *SessionManager) Logout() {
	m.launch(cmdShutdown, false)
}

func (m *SessionManager) terminate() {
	err := objLoginSessionSelf.Terminate()
	if err != nil {
		logger.Warning("LoginSessionSelf Terminate failed:", err)
	}
	os.Exit(0)
}

func (m *SessionManager) RequestLogout() {
	logger.Info("Request Logout")
	autostop.LaunchAutostopScripts(logger)

	if soundutils.CanPlayEvent(soundutils.EventDesktopLogout) {
		playLogoutSound()
	}
	m.terminate()
}

func (m *SessionManager) ForceLogout() {
	m.terminate()
}

func (shudown *SessionManager) CanShutdown() bool {
	str, _ := objLogin.CanPowerOff()
	if str == "yes" {
		return true
	}

	return false
}

func (m *SessionManager) Shutdown() {
	m.launch(cmdShutdown, false)
}

func (m *SessionManager) RequestShutdown() {
	preparePlayShutdownSound()
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
	m.launch(cmdShutdown, false)
}

func (m *SessionManager) RequestReboot() {
	preparePlayShutdownSound()
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
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	return conn.Object(lockFrontDest, lockFrontObjPath).Call(lockFrontIfc+".Show", 0).Store()
}

func (m *SessionManager) PowerOffChoose() {
	m.launch(cmdShutdown, false)
}

func (m *SessionManager) ToggleDebug() {
	if logger.GetLogLevel() == log.LevelDebug {
		doSetLogLevel(log.LevelInfo)
		logger.Debug("Debug mode disabled")
	} else {
		doSetLogLevel(log.LevelDebug)
		logger.Debug("Debug mode enabled")
	}
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
	const login1Dest = "org.freedesktop.login1"
	const login1ObjPath = "/org/freedesktop/login1"
	const login1SessionSelfObjPath = login1ObjPath + "/session/self"

	objLogin, err = login1.NewManager(login1Dest, login1ObjPath)
	if err != nil {
		panic(fmt.Errorf("new Login1 Failed: %s", err))
	}

	objLoginSessionSelf, err = login1.NewSession(login1Dest, login1SessionSelfObjPath)
	if err != nil {
		panic(fmt.Errorf("new Login1 session self Failed: %s", err))
	}

	if getSwapSchedEnabled() {
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

	sessionID := objLoginSessionSelf.Id.Get()

	err = callSwapSchedHelperPrepare(sessionID)
	if err != nil {
		logger.Warning("call SwapSchedHelper.Prepare error:", err)
	}

	swapsched.SetLogger(logger)
	swapSchedDispatcher, err = swapsched.NewDispatcher(swapsched.Config{
		UIAppsCGroup: sessionID + "@dde/uiapps",
		DECGroup:     sessionID + "@dde/DE",
	})

	if err == nil {
		// add self to DE cgroup
		deCg := cgroup.NewCgroup(swapSchedDispatcher.GetDECGroup())
		deCg.AddController(cgroup.Memory)
		err = deCg.AttachCurrentProcess()
		if err != nil {
			logger.Warning("failed to add self to DE cgroup:", err)
		}

		go swapsched.ActiveWindowHandler(swapSchedDispatcher.ActiveWindowHandler).Monitor()
		go swapSchedDispatcher.Balance()
	} else {
		logger.Warning("failed to new swap sched dispatcher:", err)
	}
}

func newSessionManager() *SessionManager {
	m := &SessionManager{}
	m.cookies = make(map[string]chan time.Time)
	m.setPropName("CurrentUid")

	return m
}

func (manager *SessionManager) launchWindowManager() {
	logger.Debug("Will launch wm")
	err := wm.Start(logger)
	if err != nil {
		logger.Error("Failed to start wm module:", err)
		return
	}
	manager.launch("env", false, "GDK_SCALE=1", wm.GetWM())
}

func exportEnvironments(env []string) {
	var willExport []string
	for _, e := range env {
		if os.Getenv(e) != "" {
			willExport = append(willExport, e)
		}
	}
	exec.Command("/usr/bin/dbus-update-activation-environment",
		append([]string{"--systemd"}, willExport...)...).Run()
}

func setupEnvironments() {
	// Fixed: Set `GNOME_DESKTOP_SESSION_ID` to cheat `xdg-open`
	// https://tower.im/projects/8162ac3745044ca29f9f3d21beaeb93d/todos/d51f8f2a317740cca3af15384d34e79f/
	os.Setenv("GNOME_DESKTOP_SESSION_ID", "this-is-deprecated")
	os.Setenv("XDG_CURRENT_DESKTOP", "Deepin")

	exportEnvironments([]string{
		"GNOME_DESKTOP_SESSION_ID",
		"XDG_CURRENT_DESKTOP",
		"SSH_AUTH_SOCK",
	})
}

func startSession(xu *xgbutil.XUtil) {
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

	setupEnvironments()

	manager.setPropStage(SessionStageInitBegin)
	manager.launchWindowManager()
	manager.setPropStage(SessionStageInitEnd)

	manager.setPropStage(SessionStageCoreBegin)
	startStartManager(xu)

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		manager.launch("/usr/bin/dde-desktop", true)
		manager.launch("/usr/bin/dde-file-manager", false, "-d")
		wg.Done()
	}()

	go func() {
		// dde-session-initializer contains backend of dock, launcher currently
		manager.launch("/usr/lib/deepin-daemon/dde-session-initializer", true)
		manager.launch("/usr/bin/dde-dock", false)
		// dde-launcher is fast enough now, there's no need to start it at the beginning
		// of every session.
		// manager.launch("/usr/bin/dde-launcher", false)
		wg.Done()
	}()

	go func() {
		manager.launch("/usr/lib/deepin-notifications/deepin-notifications", false)
		wg.Done()
	}()

	wg.Wait()

	go func() {
		// dde-session-daemon must be launched done before startAutostartProgram
		manager.launch("/usr/lib/deepin-daemon/dde-session-daemon", true)
		manager.setPropStage(SessionStageCoreEnd)

		manager.setPropStage(SessionStageAppsBegin)
		if !*debug {
			delay := getAutostartDelay()
			logger.Debug("Autostart delay seconds:", delay)
			if delay > 0 {
				time.AfterFunc(time.Second*time.Duration(delay), func() {
					startAutostartProgram()
				})
			} else {
				startAutostartProgram()
			}
		}
		manager.setPropStage(SessionStageAppsEnd)
	}()
}
