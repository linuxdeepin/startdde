package main

import (
	"flag"
	"pkg.linuxdeepin.com/lib/glib-2.0"
	"pkg.linuxdeepin.com/lib/log"
	"pkg.linuxdeepin.com/lib/proxy"
)

var logger = log.NewLogger("com.deepin.SessionManager")

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
