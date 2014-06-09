package main

import (
	"dlib/glib-2.0"
	"dlib/logger"
	"flag"
)

var Logger = logger.NewLogger("com.deepin.SessionManager")

func main() {
	var debug bool = false
	flag.BoolVar(&debug, "d", false, "debug")
	flag.Parse()

	startXSettings()

	startDisplay()

	startSession()

	startStartManager()

	if !debug {
		startAutostartProgram()
	}

	glib.StartLoop()
}
