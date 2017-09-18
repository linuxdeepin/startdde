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
	"github.com/BurntSushi/xgbutil"
	"os"
	"pkg.deepin.io/dde/startdde/display"
	"pkg.deepin.io/dde/startdde/watchdog"
	"pkg.deepin.io/lib/log"
	"pkg.deepin.io/lib/proxy"
)

var logger = log.NewLogger("startdde")

var debug = flag.Bool("d", false, "debug")
var windowManagerBin = flag.String("wm", "/usr/bin/deepin-wm-switcher", "the window manager used by dde")

func main() {
	// init x conn
	xu, err := xgbutil.NewConn()
	if err != nil {
		logger.Warning(err)
		os.Exit(1)
	}

	C.gtkInit()
	flag.Parse()
	initObjSoundThemePlayer()
	quitSoundThemePlayer()

	tryLaunchWMChooser()
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

	go display.Start()

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
