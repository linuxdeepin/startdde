package main

import (
	"flag"
	"pkg.linuxdeepin.com/lib/glib-2.0"
	"pkg.linuxdeepin.com/lib/log"
	"pkg.linuxdeepin.com/lib/proxy"
)

var logger = log.NewLogger("com.deepin.SessionManager")

var debug = flag.Bool("d", false, "debug")
var WindowManager = flag.String("wm", "compiz", "the window manager used by dde")

func main() {
	flag.Parse()

	proxy.SetupProxy()

	startXSettings()

	startDisplay()

	startSession()

	glib.StartLoop()
}
