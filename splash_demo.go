package main

import (
	"dlib"
	"dlib/logger"
)

var Logger = logger.NewLogger("demo/wallpaper")

func main() {
	initBackground()
	updateBackground(false)
	go dlib.StartLoop()
	select {}
}
