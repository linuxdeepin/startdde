package main

import (
	"pkg.linuxdeepin.com/lib/glib-2.0"
	liblogger "pkg.linuxdeepin.com/lib/logger"
	"pkg.linuxdeepin.com/lib/proxy"
	"flag"
)

var logger = liblogger.NewLogger("com.deepin.SessionManager")

var debug bool = false

func main() {
	flag.BoolVar(&debug, "d", false, "debug")
	flag.Parse()

	proxy.SetupProxy()

	startXSettings()

	startDisplay()

	startSession()

	glib.StartLoop()
}
