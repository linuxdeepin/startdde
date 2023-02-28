// SPDX-FileCopyrightText: 2014 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "C"
import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	dbus "github.com/godbus/dbus"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/startdde/display"

	// "github.com/linuxdeepin/startdde/watchdog"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/gsettings"
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-lib/proxy"
	wl_display "github.com/linuxdeepin/startdde/wl_display"
	"github.com/linuxdeepin/startdde/xsettings"
)

var logger = log.NewLogger("startdde")

var _gSettingsConfig *GSettingsConfig

var globalCgExecBin string

var globalXSManager *xsettings.XSManager

var _xConn *x.Conn

var _useWayland bool

var _useKWin bool

func init() {
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
	// TODO
	display.Init(xConn, false)
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
	var recommendedScaleFactor float64
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		logger.Info("in wayland mode")
		_useWayland = true
	}
	display.Init(xConn, _useWayland)
	// TODO
	recommendedScaleFactor = display.GetRecommendedScaleFactor()

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

	err = display.Start(service)
	if err != nil {
		logger.Warning("start display part1 failed:", err)
	}

	// 启动 display 模块的后一部分
	go func() {
		err := display.StartPart2()
		if err != nil {
			logger.Warning("start display part2 failed:", err)
		}
	}()

	go func() {
		initSoundThemePlayer()
		playLoginSound()
	}()

	err = gsettings.StartMonitor()
	if err != nil {
		logger.Warning("gsettings start monitor failed:", err)
	}
	proxy.SetupProxy()
	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		os.Exit(1)
	}
	sysSignalLoop := dbusutil.NewSignalLoop(sysBus, 10)
	sysSignalLoop.Start()

	sessionManager.start(xConn, sysSignalLoop, service)

	go func() {
		logger.Info("systemd-notify --ready")
		cmd := exec.Command("systemd-notify", "--ready")
		cmd.Run()
	}()

	service.Wait()
}

func doSetLogLevel(level log.Priority) {
	logger.SetLogLevel(level)
	if !_useWayland {
		display.SetLogLevel(level)
	} else {
		wl_display.SetLogLevel(level)
	}
	// watchdog.SetLogLevel(level)
}
