package main

import (
	"fmt"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xprop"
	// "pkg.linuxdeepin.com/lib/gio-2.0"
	"pkg.linuxdeepin.com/lib/glib-2.0"
	// dd "runtime/debug"
	"testing"
	"time"
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
		fmt.Println("screen resolution", c.w*c.h)
	}
}

func TestSplash(t *testing.T) {
	initSplash()
	initSplashAfterDependsLoaded()

	// loadBgFile()
	// drawBg()
	// mapBgToRoot()
	// bgGSettings.Connect("changed", func(s *gio.Settings, key string) {
	// 	switch key {
	// 	case gkeyCurrentBackground:
	// 		logger.Info("background value in gsettings changed:", key, getBackgroundFile())
	// 		loadBgFile()
	// 		drawBg()
	// 		mapBgToRoot()
	// 	}
	// })

	// dd.FreeOSMemory()

	go glib.StartLoop()
	time.Sleep(600 * time.Second)
}

func ManualTestReadRootPixmap(t *testing.T) {
	if drawWindowThroughRootPixmap() {
		time.Sleep(30 * time.Second)
	}
}
func drawWindowThroughRootPixmap() bool {
	ximg, err := xgraphics.NewDrawable(XU, getRootPixmap(ddeBgPixmapBlurProp))
	if err != nil {
		fmt.Println("error:", err)
		return false
	}
	ximg.XShow()
	return true
}
func getRootPixmap(prop string) (d xproto.Drawable) {
	reply, _ := xprop.GetProperty(XU, XU.RootWin(), ddeBgPixmapBlurProp)
	d = xproto.Drawable(xgb.Get32(reply.Value))
	fmt.Println("pixmap id:", d)
	return
}
