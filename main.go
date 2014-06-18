package main

import (
	"dlib/glib-2.0"
	liblogger "dlib/logger"
	"dlib/proxy"
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
