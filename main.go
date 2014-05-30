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

	startXSettings()

	// Session Manager
	startSession()
	startStartManager()

	// create background window and keep it empty
	initBackground()

	if !notStartInitPro {
		go exec.Command("/usr/bin/compiz").Run()
		<-time.After(time.Millisecond * 200)

		go exec.Command("/usr/lib/deepin-daemon/display").Run()

		initBackgroundAfterDependsLoaded()

		go exec.Command("/usr/lib/deepin-daemon/themes").Run()
		go exec.Command("/usr/lib/deepin-daemon/keybinding").Run()
		go exec.Command("/usr/lib/deepin-daemon/power").Run()
		go exec.Command("/usr/lib/deepin-daemon/inputdevices").Run()
		go exec.Command("/usr/lib/deepin-daemon/clipboard").Run()
		<-time.After(time.Millisecond * 20)

		go exec.Command("/usr/lib/deepin-daemon/dock-daemon", "-d").Run()
		<-time.After(time.Millisecond * 30)
		go exec.Command("/usr/lib/deepin-daemon/dock-apps-builder", "-d").Run()
		<-time.After(time.Millisecond * 30)

		go exec.Command("/usr/bin/dde-dock").Run()
		<-time.After(time.Millisecond * 200)

		go exec.Command("/usr/bin/dde-desktop").Run()
		<-time.After(time.Millisecond * 3000)

		go exec.Command("/usr/lib/deepin-daemon/launcher").Run()
		<-time.After(time.Millisecond * 3000)

		go exec.Command("/usr/lib/deepin-daemon/zone-settings").Run()
		go exec.Command("/usr/lib/deepin-daemon/deepin-daemon").Run()
		<-time.After(time.Millisecond * 3000)

	}

	startAutostartProgram()

	glib.StartLoop()
}
