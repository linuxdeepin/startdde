package main

import (
	"dlib/logger"
	"flag"
	"fmt"
	"os/exec"
	"time"

	"dlib"
)

func testStartManager() {
	startStartManager()
	// args := make([]*gio.File, 0)
	// for _, name := range m.AutostartList() {
	// 	fmt.Println("launch", name)
	// 	m.Launch(name, args)
	// }
	dlib.StartLoop()
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
	fmt.Println("debug:", debug)
	fmt.Println("notStartInitPro:", notStartInitPro)

	startXSettings()

	// Session Manager
	startSession()
	background()

	if !notStartInitPro {
		go exec.Command("/usr/bin/compiz").Run()
		<-time.After(time.Millisecond * 200)

		go exec.Command("/usr/lib/deepin-daemon/keybinding").Run()
		go exec.Command("/usr/lib/deepin-daemon/display").Run()
		<-time.After(time.Millisecond * 20)

		go exec.Command("/usr/bin/dock").Run()
		<-time.After(time.Millisecond * 200)
		go exec.Command("/usr/bin/dapptray").Run()
		<-time.After(time.Millisecond * 20)

		go exec.Command("/usr/bin/desktop").Run()
		<-time.After(time.Millisecond * 3000)

		go exec.Command("/usr/lib/deepin-daemon/launcher-daemon").Run()
		<-time.After(time.Millisecond * 3000)
		
        go exec.Command("/usr/lib/deepin-daemon/osd").Run()
		<-time.After(time.Millisecond * 3000)
	}

	startStartManager()

	for {
		<-time.After(time.Millisecond * 1000)
	}
}
