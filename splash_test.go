package main

import (
	"dlib"
	"testing"
	// "time"
	"fmt"
)

func TestGetPrimaryScreenResolution(t *testing.T) {
	tests := []struct {
		w, h, r uint16
	}{
		{1024, 768, 0},
		{1440, 900, 50806},
		{1280, 1024, 0},
	}
	for _, c := range tests {
		fmt.Println(c.w * c.h)
	}
}

func TestSplash(t *testing.T) {
	initBackground()
	initBackgroundAfterDependsLoaded()
	go dlib.StartLoop()
	// time.Sleep(10 * time.Second)
	select {}
}
