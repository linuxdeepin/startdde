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

import "C"
import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	dbus "github.com/godbus/dbus"
	accounts "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.accounts"
	notifications "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.notifications"
	x "github.com/linuxdeepin/go-x11-client"
	"pkg.deepin.io/dde/startdde/display"
	"pkg.deepin.io/dde/startdde/iowait"
	"pkg.deepin.io/dde/startdde/watchdog"
	wl_display "pkg.deepin.io/dde/startdde/wl_display"
	"pkg.deepin.io/dde/startdde/wm_kwin"
	"pkg.deepin.io/dde/startdde/xsettings"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/gettext"
	"pkg.deepin.io/lib/gsettings"
	"pkg.deepin.io/lib/log"
	"pkg.deepin.io/lib/proxy"
)

var logger = log.NewLogger("startdde")

var _options struct {
	noXSessionScripts bool
}

var _gSettingsConfig *GSettingsConfig

var globalCgExecBin string

var globalXSManager *xsettings.XSManager

var _xConn *x.Conn

var _useWayland bool

var _inVM bool

var _useKWin bool

var _homeDir string

func init() {
	flag.BoolVar(&_options.noXSessionScripts, "no-xsession-scripts", false, "")
}

func reapZombies() {
	// We must reap children process even we hasn't create anyone at this moment,
	// Because the startdde may be launched by exec syscall
	// in another existed process, like /usr/sbin/lighdm-session does.
	// NOTE: Don't use signal.Ignore(syscall.SIGCHILD), otherwise os/exec wouldn't work properly.
	//       And simply ignore SIGCHILD hasn't any helpful in here.
	for {
		pid, err := syscall.Wait4(-1, nil, syscall.WNOHANG, nil)
		if err != nil || pid == 0 {
			break
		}
	}
}

func shouldUseDDEKWin() bool {
	_, err := os.Stat("/usr/bin/kwin_no_scale")
	return err == nil
}

const (
	cmdKWin                = "/usr/bin/kwin_no_scale"
	cmdDdeSessionDaemon    = "/usr/lib/deepin-daemon/dde-session-daemon"
	cmdDdeDock             = "/usr/bin/dde-dock"
	cmdDdeDesktop          = "/usr/bin/dde-desktop"
	cmdLoginReminderHelper = "/usr/libexec/deepin/login-reminder-helper"
	cmdDdeHintsDialog      = "/usr/bin/dde-hints-dialog"

	loginReminderTimeout    = 5 * time.Second
	loginReminderTimeFormat = "2006-01-02 15:04:05"
	secondsPerDay           = 60 * 60 * 24
	accountUserPath         = "/com/deepin/daemon/Accounts/User"
)

func launchCoreComponents(sm *SessionManager) {
	setupEnvironments1()

	wmChooserLaunched := false
	if !_useWayland && _inVM {
		wmChooserLaunched = maybeLaunchWMChooser()
	}

	const waitDelayDuration = 7 * time.Second

	var wg sync.WaitGroup
	launch := func(program string, args []string, name string, wait bool, endFn func()) {
		if wait {
			wg.Add(1)
			sm.launchWaitCore(name, program, args, waitDelayDuration, func(launchOk bool) {
				if endFn != nil {
					endFn()
				}
				wg.Done()
			})
			return
		}

		sm.launchWithoutWait(program, args...)
	}

	coreStartTime := time.Now()
	// launch window manager
	if !_useWayland {
		_useKWin = shouldUseDDEKWin()
		if _useKWin {
			if wmChooserLaunched {
				wm_kwin.SyncWmChooserChoice()
			}
			launch(cmdKWin, nil, "kwin", true, func() {
				// 等待 kwin 就绪，然后让 dock 显示
				handleKWinReady(sm)
			})
		} else {
			wmCmd := _gSettingsConfig.wmCmd
			if wmCmd == "" {
				wmCmd = "x-window-manager"
			}
			launch("env", []string{"GDK_SCALE=1", wmCmd}, "wm", false, nil)
		}
	} else {
		var handleId dbusutil.SignalHandlerId
		var needDel = true
		handleId, err := sm.dbusDaemon.ConnectNameOwnerChanged(func(name, newOwner, oldOwner string) {
			if name == "org.kde.KWin" && newOwner != "" && oldOwner == "" {
				handleKWinReady(sm)
				sm.dbusDaemon.RemoveHandler(handleId)
			}
		})
		if err != nil {
			logger.Warningf("connect to signal err: %s, run handle after timeout", err)
			time.AfterFunc(time.Second*5, func() {
				handleKWinReady(sm)
			})
			needDel = false
		}

		hasOwner, err := sm.dbusDaemon.NameHasOwner(0, "org.kde.KWin")
		if err != nil {
			logger.Warning(err)
			return
		}

		if hasOwner {
			handleKWinReady(sm)
			if needDel {
				sm.dbusDaemon.RemoveHandler(handleId)
			}
		}
	}

	// 先启动 dde-session-daemon，再启动 dde-dock
	launch(cmdDdeSessionDaemon, nil, "dde-session-daemon", true, func() {
		var dockArgs []string
		if _useKWin {
			dockArgs = []string{"-r"}
		}
		launch(cmdDdeDock, dockArgs, "dde-dock", true, nil)
	})
	launch(cmdDdeDesktop, nil, "dde-desktop", true, nil)

	wg.Wait()
	logger.Info("core components cost:", time.Since(coreStartTime))
}

func handleKWinReady(sm *SessionManager) {
	sessionBus := sm.service.Conn()

	const dockServiceName = "com.deepin.dde.Dock"
	callDockShow := func() {
		logInfoAfter("call com.deepin.dde.Dock callShow")
		dockObj := sessionBus.Object(dockServiceName, "/com/deepin/dde/Dock")
		ctx, cancelFn := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelFn()
		err := dockObj.CallWithContext(ctx, dockServiceName+".callShow", dbus.FlagNoAutoStart).Err
		if err != nil {
			logger.Warning("call dde-dock callShow failed:", err)
		}
	}

	var sigHandleId dbusutil.SignalHandlerId
	sigHandleId, err := sm.dbusDaemon.ConnectNameOwnerChanged(func(name string, oldOwner string, newOwner string) {
		if name == dockServiceName && oldOwner == "" && newOwner != "" {
			callDockShow()
			sm.dbusDaemon.RemoveHandler(sigHandleId)
		}
	})
	has, err := sm.dbusDaemon.NameHasOwner(0, dockServiceName)
	if err != nil {
		logger.Warning(err)
	} else if has {
		callDockShow()
		sm.dbusDaemon.RemoveHandler(sigHandleId)
	}
}

var _mainBeginTime time.Time

func logDebugAfter(msg string) {
	elapsed := time.Since(_mainBeginTime)
	logger.Debugf("after %s, %s", elapsed, msg)
}

func logInfoAfter(msg string) {
	elapsed := time.Since(_mainBeginTime)
	logger.Infof("after %s, %s", elapsed, msg)
}

func greeterDisplayMain() {
	display.SetGreeterMode(true)
	// init x conn
	xConn, err := x.NewConn()
	if err != nil {
		logger.Warning(err)
		os.Exit(1)
	}
	display.Init(xConn)
	logger.Debug("greeter mode")
	service, err := dbusutil.NewSessionService()
	if err != nil {
		logger.Warning(err)
	}
	err = display.Start(service)
	if err != nil {
		logger.Warning(err)
	}
	err = display.StartPart2()
	if err != nil {
		logger.Warning(err)
	}
	service.Wait()
}

func main() {
	flag.Parse()
	if len(os.Args) > 0 && strings.HasPrefix(filepath.Base(os.Args[0]), "greeter") {
		// os.Args[0] 应该一般是 greeter-display-daemon
		greeterDisplayMain()
		return
	}

	initGSettingsConfig()

	_mainBeginTime = time.Now()

	gettext.InitI18n()
	gettext.BindTextdomainCodeset("startdde", "UTF-8")
	gettext.Textdomain("startdde")

	reapZombies()
	// init x conn
	xConn, err := x.NewConn()
	if err != nil {
		logger.Warning(err)
		os.Exit(1)
	}
	_xConn = xConn
	if _options.noXSessionScripts {
		runXSessionScriptsFaster(xConn)
	}
	_inVM, err = isInVM()
	if err != nil {
		logger.Warning("detect VM failed:", err)
	}
	var recommendedScaleFactor float64
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		logger.Info("in wayland mode")
		_useWayland = true
		// 相比于 X11 环境，在 Wayland 环境下，先于启动核心组件之前启动了 wl_display 模块。
		err := wl_display.Start()
		if err != nil {
			logger.Warning(err)
		}
		recommendedScaleFactor = wl_display.GetRecommendedScaleFactor()
	} else {
		display.Init(xConn)
		recommendedScaleFactor = display.GetRecommendedScaleFactor()
	}

	service, err := dbusutil.NewSessionService()
	if err != nil {
		logger.Warning(err)
		os.Exit(1)
	}

	xsManager, err := xsettings.Start(xConn, recommendedScaleFactor, service, &display.ScaleFactorsHelper)
	if err != nil {
		logger.Warning(err)
	} else {
		globalXSManager = xsManager
	}

	sessionManager := newSessionManager(service)
	err = service.Export(sessionManagerPath, sessionManager)
	if err != nil {
		logger.Warning("export session sessionManager failed:", err)
	}
	err = service.RequestName(sessionManagerServiceName)
	if err != nil {
		logger.Warningf("request name %q failed: %v", sessionManagerServiceName, err)
	}
	logDebugAfter("before launchCoreComponents")

	if !_useWayland {
		// 使用 X11 环境时, 把 display 模块的启动分成两个部分，前一部分在 core components 启动之前启动，
		// 后一部分在 core components 启动之后启动。
		err := display.Start(service)
		if err != nil {
			logger.Warning("start display part1 failed:", err)
		}
	}

	launchCoreComponents(sessionManager)

	if !_useWayland {
		// 启动 display 模块的后一部分
		go func() {
			err := display.StartPart2()
			if err != nil {
				logger.Warning("start display part2 failed:", err)
			}
		}()
	}

	go func() {
		initSoundThemePlayer()
		playLoginSound()
	}()

	err = gsettings.StartMonitor()
	if err != nil {
		logger.Warning("gsettings start monitor failed:", err)
	}
	proxy.SetupProxy()

	if _inVM {
		time.AfterFunc(10*time.Second, func() {
			logger.Debug("try to correct vm resolution")
			correctVMResolution()
		})
	}

	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		os.Exit(1)
	}
	sysSignalLoop := dbusutil.NewSignalLoop(sysBus, 10)
	sysSignalLoop.Start()

	sessionManager.start(xConn, sysSignalLoop, service)
	watchdog.Start(sessionManager.getLocked, _useKWin)

	if _gSettingsConfig.iowaitEnabled {
		go iowait.Start(logger)
	} else {
		logger.Info("iowait disabled")
	}

	go handleOSSignal(sessionManager)

	loginReminder()

	service.Wait()
}

func handleOSSignal(m *SessionManager) {
	var sigChan = make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGABRT, syscall.SIGTERM, syscall.SIGSEGV)

loop:
	for sig := range sigChan {
		switch sig {
		case syscall.SIGINT, syscall.SIGABRT, syscall.SIGTERM, syscall.SIGSEGV:
			logger.Info("received signal:", sig)
			break loop
		}
	}

	logger.Info("received unexpected signal, force logout")
	m.doLogout(true)
}

func doSetLogLevel(level log.Priority) {
	logger.SetLogLevel(level)
	if !_useWayland {
		display.SetLogLevel(level)
	} else {
		wl_display.SetLogLevel(level)
	}
	watchdog.SetLogLevel(level)
}

func loginReminder() {
	if !_gSettingsConfig.loginReminder {
		return
	}

	sysBus, _ := dbus.SystemBus()

	uid := os.Getuid()
	userPath := accountUserPath + strconv.Itoa(uid)

	user, err := accounts.NewUser(sysBus, dbus.ObjectPath(userPath))
	if err != nil {
		logger.Warning("failed to get user:", err)
	}

	res, err := user.GetReminderInfo(0)
	if err != nil {
		logger.Warning("failed to get reminder info:", err)
	}

	currentLoginTime, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", res.CurrentLogin.Time)
	if err != nil {
		logger.Warning("wrong time format:", err)
	}
	currentLoginTimeStr := currentLoginTime.Format(loginReminderTimeFormat)

	address := res.CurrentLogin.Address
	if address == "0.0.0.0" {
		// 若 IP 是「0.0.0.0」，获取当前设备的 IP
		address = getFirstIPAddress()
	}

	body := fmt.Sprintf("%s %s %s", res.Username, address, currentLoginTimeStr)

	// pam_unix/passverify.c
	curDays := int(time.Now().Unix() / secondsPerDay)
	daysLeft := res.Spent.LastChange + res.Spent.Max - curDays
	if res.Spent.Max != -1 && res.Spent.Warn != -1 {
		if res.Spent.Warn > daysLeft {
			body += " " + fmt.Sprintf(gettext.Tr("Your password will expire in %d days"), daysLeft)
		}
	}

	body += "\n" + fmt.Sprintf(gettext.Tr("%d login failures since the last successful login"), res.FailCountSinceLastLogin)

	bus, _ := dbus.SessionBus()
	notifi := notifications.NewNotifications(bus)
	sigLoop := dbusutil.NewSignalLoop(bus, 10)
	sigLoop.Start()
	notifi.InitSignalExt(sigLoop, true)

	// TODO: icon
	title := gettext.Tr("Login Reminder")
	actions := []string{"details", gettext.Tr("Details")}
	notifyId, err := notifi.Notify(0, "dde-control-center", 0, "preferences-system", title, body, actions, nil, 0)
	if err != nil {
		logger.Warningf("failed to send notify: %s", err)
		return
	}

	_, err = notifi.ConnectActionInvoked(func(id uint32, actionKey string) {
		if id != notifyId || actionKey != "details" {
			return
		}

		lastLoginTime, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", res.LastLogin.Time)
		if err != nil {
			logger.Warning("wrong time format:", err)
		}
		lastLoginTimeStr := lastLoginTime.Format(loginReminderTimeFormat)

		content := fmt.Sprintf("<p>%s</p>", res.Username)
		content += fmt.Sprintf("<p>%s</p>", address)
		content += "<p>" + fmt.Sprintf(gettext.Tr("Login time: %s"), currentLoginTimeStr) + "</p>"
		content += "<p>" + fmt.Sprintf(gettext.Tr("Last login: %s"), lastLoginTimeStr) + "</p>"
		content += "<p><b>" + fmt.Sprintf(gettext.Tr("Your password will expire in %d days"), daysLeft) + "</b></p>"
		content += "<br>"
		content += "<p>" + fmt.Sprintf(gettext.Tr("%d login failures since the last successful login"), res.FailCountSinceLastLogin) + "</p>"

		cmd := exec.Command(cmdDdeHintsDialog, title, content)
		err = cmd.Start()
		if err != nil {
			logger.Warning("failed to start dde-hints-dialog:", err)
			return
		}

		go func() {
			err = cmd.Wait()
			if err != nil {
				logger.Warning("failed to run dde-hints-dialog", err)
				return
			}
		}()
	})
	if err != nil {
		logger.Warning("connect to ActionInvoked failed:", err)
	}

	// 通知不显示在通知中心面板，故在时间到了后，关闭通知
	time.AfterFunc(loginReminderTimeout, func() {
		notifi.CloseNotification(0, notifyId)

		notifi.RemoveAllHandlers()

		sigLoop.Stop()
	})
}

func getFirstIPAddress() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	for _, iface := range ifaces {
		if iface.Name == "lo" {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			logger.Warningf("failed to get address of %s: %s", iface.Name, err)
			continue
		}

		for _, addr := range addrs {
			// remove the netmask
			return strings.Split(addr.String(), "/")[0]
		}
	}

	return "127.0.0.1"
}
