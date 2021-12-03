package main

import (
	"fmt"
	"os"

	wl_display "github.com/linuxdeepin/startdde/wl_display"
)

func main() {
	err := wl_display.Start()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	select {}
}
