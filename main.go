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
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	dbus "github.com/godbus/dbus"
	x "github.com/linuxdeepin/go-x11-client"
	"pkg.deepin.io/dde/startdde/display"
	"pkg.deepin.io/dde/startdde/iowait"
	"pkg.deepin.io/dde/startdde/watchdog"
	wl_display "pkg.deepin.io/dde/startdde/wl_display"
	"pkg.deepin.io/dde/startdde/wm_kwin"
	"pkg.deepin.io/dde/startdde/xsettings"
	"pkg.deepin.io/lib/dbusutil"
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
	cmdKWin             = "/usr/bin/kwin_no_scale"
	cmdDdeSessionDaemon = "/usr/lib/deepin-daemon/dde-session-daemon"
	cmdDdeDock          = "/usr/bin/dde-dock"
	cmdDdeDesktop       = "/usr/bin/dde-desktop"
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
			launch(cmdKWin, nil, "kwin", true, nil)
		} else {
			wmCmd := _gSettingsConfig.wmCmd
			if wmCmd != "" {
				launch("env", []string{"GDK_SCALE=1", wmCmd}, "wm", false, nil)
			}
		}
	}

	launch(cmdDdeDesktop, nil, "dde-desktop", true, nil)
	// 先启动 dde-session-daemon，再启动 dde-dock
	launch(cmdDdeSessionDaemon, nil, "dde-session-daemon", true, func() {
		launch(cmdDdeDock, nil, "dde-dock", true, nil)
	})

	wg.Wait()
	logger.Info("core components cost:", time.Since(coreStartTime))
}

var _mainBeginTime time.Time

func logDebugAfter(msg string) {
	elapsed := time.Since(_mainBeginTime)
	logger.Debugf("after %s, %s", elapsed, msg)
}

func main() {
	_mainBeginTime = time.Now()
	flag.Parse()
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

	xsManager, err := xsettings.Start(xConn, logger, recommendedScaleFactor, service)
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

	var displayStartedCh chan struct{}
	if !_useWayland {
		displayStartedCh = make(chan struct{})
		// 使用 X11 环境时, 把 display 模块的启动分成两个部分，前一部分和 core components 一起启动，
		// 后一部分在 core components 启动之后启动。
		go func() {
			display.Start(service)
			displayStartedCh <- struct{}{}
		}()
	}

	launchCoreComponents(sessionManager)

	if !_useWayland {
		go func() {
			<-displayStartedCh
			err := display.StartPart2(service)
			if err != nil {
				logger.Warning("start display part2 failed:", err)
			}
		}()
	}

	_gSettingsConfig = getGSettingsConfig()

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

	go handleOSSignal()
	service.Wait()
}

func handleOSSignal() {
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
	doLogout(true)
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
