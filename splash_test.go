package main

import (
	"dlib"
	"testing"
	"time"
)

func TestSplash(t *testing.T) {
	initBackground()
	updateBackground(false)
	go dlib.StartLoop()
	time.Sleep(10 * time.Second)
}
