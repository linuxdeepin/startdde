package main

import (
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
)

func main() {
	// test()
	// return

	flag.BoolVar(&debug, "d", false, "debug")
	flag.BoolVar(&notStartInitPro, "n", false, "not start")

	flag.Parse()
	fmt.Println("debug:", debug)
	fmt.Println("notStartInitPro:", notStartInitPro)

	if !notStartInitPro {
		go exec.Command("/usr/bin/compiz").Run()
		<-time.After(time.Millisecond * 200)

		go exec.Command("/usr/bin/gnome-settings-daemon").Run()
		<-time.After(time.Millisecond * 100)

		go exec.Command("/usr/lib/deepin-daemon/keybinding").Run()
		go exec.Command("/usr/lib/deepin-daemon/themes").Run()
		go exec.Command("/usr/lib/deepin-daemon/display").Run()
		<-time.After(time.Millisecond * 20)

		go exec.Command("/usr/bin/dock").Run()
		<-time.After(time.Millisecond * 200)
		go exec.Command("/usr/bin/dapptray").Run()
		<-time.After(time.Millisecond * 20)

		go exec.Command("/usr/bin/desktop").Run()
		<-time.After(time.Millisecond * 3000)

		<-time.After(time.Millisecond * 3000)

		// Session Manager
		startSession()
	}
	startStartManager()

	for {
		<-time.After(time.Millisecond * 1000)
	}
}
