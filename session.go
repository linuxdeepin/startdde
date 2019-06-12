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
	"strconv"
	"sync"
	"time"

	"github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	"github.com/linuxdeepin/go-x11-client"
	"pkg.deepin.io/dde/api/soundutils"
	"pkg.deepin.io/dde/startdde/autostop"
	"pkg.deepin.io/dde/startdde/keyring"
	"pkg.deepin.io/dde/startdde/memchecker"
	"pkg.deepin.io/dde/startdde/swapsched"
	"pkg.deepin.io/dde/startdde/watchdog"
	"pkg.deepin.io/dde/startdde/wm"
	"pkg.deepin.io/dde/startdde/wm_kwin"
	"pkg.deepin.io/dde/startdde/xcursor"
	"pkg.deepin.io/dde/startdde/xsettings"
	"pkg.deepin.io/gir/gio-2.0"
	"pkg.deepin.io/lib/cgroup"
	"pkg.deepin.io/lib/dbus"
	dbus1 "pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/keyfile"
	"pkg.deepin.io/lib/log"
	"pkg.deepin.io/lib/procfs"
	"pkg.deepin.io/lib/xdg/basedir"
)

type SessionManager struct {
	mu                    sync.Mutex
	Locked                bool
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

	err = objLoginSessionSelf.Terminate(0)
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
	str, _ := objLogin.CanPowerOff(0)
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
	objLogin.PowerOff(0, true)
}

func (m *SessionManager) ForceShutdown() {
	objLogin.PowerOff(0, false)
}

func (shudown *SessionManager) CanReboot() bool {
	str, _ := objLogin.CanReboot(0)
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
	objLogin.Reboot(0, true)
}

func (m *SessionManager) ForceReboot() {
	objLogin.Reboot(0, false)
}

func (m *SessionManager) CanSuspend() bool {
	_, err := os.Stat("/sys/power/mem_sleep")
	if os.IsNotExist(err) {
		return false
	}

	str, _ := objLogin.CanSuspend(0)
	if str == "yes" {
		return true
	}
	return false
}

func (m *SessionManager) RequestSuspend() {
	objLogin.Suspend(0, false)
}

func (m *SessionManager) CanHibernate() bool {
	str, _ := objLogin.CanHibernate(0)
	if str == "yes" {
		return true
	}
	return false
}

func (m *SessionManager) RequestHibernate() {
	objLogin.Hibernate(0, false)
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

func (m *SessionManager) SetLocked(dMsg dbus.DMessage, value bool) error {
	pid := dMsg.GetSenderPID()
	process := procfs.Process(pid)
	exe, err := process.Exe()
	if err != nil {
		return err
	}

	if exe != "/usr/bin/dde-lock" {
		return fmt.Errorf("exe %q is invalid", exe)
	}

	m.mu.Lock()
	if m.Locked != value {
		m.Locked = value
		dbus.NotifyChange(m, "Locked")
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

	return nil
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

func initSession() {
	var err error
	const login1ObjPath = "/org/freedesktop/login1"
	const login1SessionSelfObjPath = login1ObjPath + "/session/self"

	sysBus, err := dbus1.SystemBus()
	if err != nil {
		logger.Warning(err)
		return
	}

	objLogin = login1.NewManager(sysBus)
	objLoginSessionSelf, err = login1.NewSession(sysBus, login1SessionSelfObjPath)
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

	sessionID, err := objLoginSessionSelf.Id().Get(0)
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

func (manager *SessionManager) launchWindowManager(useKwin bool) {
	logger.Debug("Will launch wm")
	if useKwin {
		err := wm_kwin.Start(logger, globalWmChooserLaunched)
		if err != nil {
			logger.Warning(err)
		}
		manager.launch("kwin_no_scale", true)
		return
	}

	err := wm.Start(logger, globalWmChooserLaunched)
	if err != nil {
		logger.Error("Failed to start wm module:", err)
		return
	}
	manager.launch("env", wm.ShouldWait(), "GDK_SCALE=1", wm.GetWM())
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

	scaleFactor := 1.0
	if globalXSManager != nil {
		scaleFactor = globalXSManager.GetScaleFactor()
	}
	envVars["QT_DBL_CLICK_DIST"] = strconv.Itoa(int(15 * scaleFactor))

	// set scale factor for deepin wine apps
	if scaleFactor != 1.0 {
		envVars[xsettings.EnvDeepinWineScale] = strconv.FormatFloat(
			scaleFactor, 'f', 2, 64)
	}

	for key, value := range envVars {
		logger.Debugf("set env %s = %q", key, value)
		err = os.Setenv(key, value)
		if err != nil {
			logger.Warning(err)
		}
	}

	for _, envName := range []string{
		"LANG",
		"LANGUAGE",
	} {
		envValue, ok := os.LookupEnv(envName)
		if ok {
			envVars[envName] = envValue
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

func startSession(conn *x.Conn, useKwin bool) *SessionManager {
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
		return nil
	}

	setupEnvironments()

	manager.setPropStage(SessionStageInitBegin)
	manager.launchWindowManager(useKwin)
	manager.setPropStage(SessionStageInitEnd)

	manager.setPropStage(SessionStageCoreBegin)
	startStartManager(conn)
	manager.launchDDE()
	go func() {
		setLeftPtrCursor()
		err := keyring.CheckLogin()
		if err != nil {
			logger.Warning("Failed to init keyring:", err)
		}
	}()
	go manager.launchAutostart()

	return manager
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
