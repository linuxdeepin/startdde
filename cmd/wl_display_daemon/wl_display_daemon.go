package main

import (
	"fmt"
	"os"

	wl_display "pkg.deepin.io/dde/startdde/wl_display"
)

func main() {
	err := wl_display.Start()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	select {}
}
