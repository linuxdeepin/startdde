package main

import "os/exec"
import "time"

func main() {
	go exec.Command("/usr/bin/compiz").Run()
	<-time.After(time.Millisecond * 200)

	go exec.Command("/usr/bin/gnome-settings-daemon").Run()
	<-time.After(time.Millisecond * 100)

	go exec.Command("/usr/lib/deepin-daemon/binding-manager").Run()
	go exec.Command("/usr/lib/deepin-daemon/individuate").Run()
	go exec.Command("/usr/lib/deepin-daemon/display").Run()
	<-time.After(time.Millisecond * 20)

	go exec.Command("/usr/bin/dock").Run()
	<-time.After(time.Millisecond * 200)
	go exec.Command("/usr/bin/dapptray").Run()
	<-time.After(time.Millisecond * 20)

	go exec.Command("/usr/bin/desktop").Run()
	<-time.After(time.Millisecond * 3000)

	go exec.Command("/usr/bin/dss").Run()
	go exec.Command("/usr/bin/launcher", "-H").Run()

	<-time.After(time.Millisecond * 3000)
	go exec.Command("/usr/bin/skype").Run()

        // Session Manager
        startSession()

	for {
		<-time.After(time.Millisecond * 1000)
	}
}
