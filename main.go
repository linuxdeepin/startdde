package main

import (
	"dlib/glib-2.0"
	"dlib/logger"
	"flag"
	"os"
	"os/exec"
	"time"
)

func testStartManager() {
	startStartManager()
	// args := make([]*gio.File, 0)
	// for _, name := range m.AutostartList() {
	// 	Logger.Debug("launch", name)
	// 	m.Launch(name, args)
	// }
	glib.StartLoop()
}
func init() {
	os.Setenv("DE", "DDE")
}

func test() {
	testStartManager()
}

var (
	debug           bool = false
	notStartInitPro bool = false

	Logger = logger.NewLogger("com.deepin.SessionManager")
)

func main() {
	// test()
	// return

	flag.BoolVar(&debug, "d", false, "debug")
	flag.BoolVar(&notStartInitPro, "n", false, "not start")

	flag.Parse()
	Logger.Info("debug:", debug)
	Logger.Info("notStartInitPro:", notStartInitPro)
	if debug {
		Logger.SetLogLevel(logger.LEVEL_DEBUG)
	}

	startProxy()
	startXSettings()

	// Session Manager
	startSession()
	startStartManager()

	startDisplay()

	// create background window and keep it empty
	initBackground()

	if !notStartInitPro {
		go exec.Command("/usr/bin/gtk-window-decorator").Run()
		go exec.Command("/usr/bin/compiz").Run()
		<-time.After(time.Millisecond * 200)

		initBackgroundAfterDependsLoaded()

		go exec.Command("/usr/lib/deepin-daemon/dde-session-daemon").Run()
		<-time.After(time.Millisecond * 300)

		go exec.Command("/usr/bin/dde-desktop").Run()
		<-time.After(time.Millisecond * 300)

		go exec.Command("/usr/lib/deepin-daemon/inputdevices").Run()
		<-time.After(time.Millisecond * 300)
	}

	startAutostartProgram()

	glib.StartLoop()
}
