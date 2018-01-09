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

// #cgo pkg-config: x11 gtk+-3.0
// #include <X11/Xlib.h>
// #include <gtk/gtk.h>
// void gtkInit() {
//    XInitThreads();
//    gtk_init(NULL, NULL);
// }
import "C"
import (
	"flag"
	"os"
	"syscall"

	"github.com/BurntSushi/xgbutil"
	"pkg.deepin.io/dde/startdde/display"
	"pkg.deepin.io/dde/startdde/watchdog"
	"pkg.deepin.io/lib/log"
	"pkg.deepin.io/lib/proxy"
)

var logger = log.NewLogger("startdde")

var debug = flag.Bool("d", false, "debug")

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

func main() {
	reapZombies()

	// init x conn
	xu, err := xgbutil.NewConn()
	if err != nil {
		logger.Warning(err)
		os.Exit(1)
	}

	C.gtkInit()
	flag.Parse()
	initSoundThemePlayer()

	tryMatchVM()
	go playLoginSound()

	canLaunch := canLaunchWelcome()
	if canLaunch {
		err = showWelcome(true)
		if err != nil {
			logger.Warning("Failed to show welcome:", err)
		}
	}

	proxy.SetupProxy()

	startXSettings(xu.Conn())

	go func() {
		display.Start()
		inVM, _ := isInVM()
		if inVM {
			logger.Debug("try to correct vm resolution")
			correctVMResolution()
		}
	}()

	startSession(xu)

	if canLaunch {
		err = showWelcome(false)
		if err != nil {
			logger.Warning("Failed to exit welcome:", err)
		}
	}

	watchdog.Start()

	C.gtk_main()
}

func doSetLogLevel(level log.Priority) {
	logger.SetLogLevel(level)
	display.SetLogLevel(level)
	watchdog.SetLogLevel(level)
}
