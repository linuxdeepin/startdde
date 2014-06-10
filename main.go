package main

import (
	"dlib/glib-2.0"
	"dlib/logger"
	"flag"
)

var Logger = logger.NewLogger("com.deepin.SessionManager")

var debug bool = false

func main() {
	flag.BoolVar(&debug, "d", false, "debug")
	flag.Parse()

	startProxy()

	startXSettings()

	startDisplay()

	startSession()

	glib.StartLoop()
}
