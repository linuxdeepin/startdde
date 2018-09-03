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
	"path/filepath"
	"sort"
	"sync"
	"time"

	"dbus/org/freedesktop/login1"

	"github.com/linuxdeepin/go-x11-client"
	"pkg.deepin.io/dde/api/soundutils"
	"pkg.deepin.io/dde/startdde/autostop"
	"pkg.deepin.io/dde/startdde/keyring"
	"pkg.deepin.io/dde/startdde/memchecker"
	"pkg.deepin.io/dde/startdde/swapsched"
	"pkg.deepin.io/dde/startdde/wm"
	"pkg.deepin.io/dde/startdde/xsettings"
	"pkg.deepin.io/lib/cgroup"
	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/keyfile"
	"pkg.deepin.io/lib/log"
	"pkg.deepin.io/lib/xdg/basedir"
)

type SessionManager struct {
	CurrentUid            string
	cookieLocker          sync.Mutex
	cookies               map[string]chan time.Time
	Stage                 int32
	allowSessionDaemonRun bool
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

	sessionID := objLoginSessionSelf.Id.Get()

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
	err := wm.Start(logger, globalWmChooserLaunched)
	if err != nil {
		logger.Error("Failed to start wm module:", err)
		return
	}
	manager.launch("env", true, "GDK_SCALE=1", wm.GetWM())
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
	} else if osdRunning && globalXSManager.NeedRestartOSD() {
		// restart osd
		m.launch("/usr/lib/deepin-daemon/dde-osd", false)
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
	envVars[xsettings.EnvJavaOptions] = os.Getenv(xsettings.EnvJavaOptions)
	envVars[xsettings.EnvQtScaleFactor] = os.Getenv(xsettings.EnvQtScaleFactor)

	for key, value := range envVars {
		err = os.Setenv(key, value)
		if err != nil {
			logger.Warning(err)
		}
	}

	sessionBus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return
	}

	// NOTE: since dbus-daemon --session launch with the --systemd-activation option,
	// there is no need to call systemd's SetEnvironment method.
	err = sessionBus.BusObject().Call("org.freedesktop.DBus."+
		"UpdateActivationEnvironment", 0, envVars).Err
	if err != nil {
		logger.Warning(err)
	}
}

func startSession(conn *x.Conn) {
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
	startStartManager(conn)
	manager.launchDDE()
	go func() {
		err := keyring.CheckLogin()
		if err != nil {
			logger.Warning("Failed to init keyring:", err)
		}
	}()
	go manager.launchAutostart()
}

func isDeepinVersionChanged() (bool, error) {
	kfDeepinVersion := keyfile.NewKeyFile()
	err := kfDeepinVersion.LoadFromFile("/etc/deepin-version")
	if err != nil {
		return false, err
	}

	v0, err := kfDeepinVersion.GetString("Release", "Version")
	if err != nil {
		return false, err
	}

	kfDDEWelcome := keyfile.NewKeyFile()
	ddeWelcomeFile := filepath.Join(basedir.GetUserConfigDir(), "deepin/dde-welcome.conf")

	saveDDEWelcome := func() {
		kfDDEWelcome.SetString("General", "Version", v0)
		os.MkdirAll(filepath.Dir(ddeWelcomeFile), 0755)
		err = kfDDEWelcome.SaveToFile(ddeWelcomeFile)
		if err != nil {
			logger.Warning("failed to save dde-welcome.conf:", err)
		}
	}

	err = kfDDEWelcome.LoadFromFile(ddeWelcomeFile)
	if err != nil {
		if os.IsNotExist(err) {
			// new user first login
			saveDDEWelcome()
			return false, nil
		}
		return false, err
	}

	v1, _ := kfDDEWelcome.GetString("General", "Version")

	if v0 != v1 {
		saveDDEWelcome()
		return true, nil
	}
	return false, nil
}
