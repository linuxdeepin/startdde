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
	"github.com/BurntSushi/xgbutil/ewmh"
	"testing"
	"time"
)

func ManualTestSplash(t *testing.T) {
	initSplash()
	initSplashAfterDependsLoaded()

	// TODO
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
	if drawWindowThroughRootPixmap(ddeBgWindowProp) {
		time.Sleep(300 * time.Second)
	}
}
func drawWindowThroughRootPixmap(prop string) bool {
	ximg, err := xgraphics.NewDrawable(XU, getRootPixmap(prop))
	if err != nil {
		fmt.Println("error:", err)
		return false
	}
	win := ximg.XShow()
	ewmh.WmWindowTypeSet(XU, win.Id, []string{"_NET_WM_WINDOW_TYPE_DESKTOP"})
	return true
}
func getRootPixmap(prop string) (d xproto.Drawable) {
	reply, _ := xprop.GetProperty(XU, XU.RootWin(), prop)
	d = xproto.Drawable(xgb.Get32(reply.Value))
	fmt.Println("pixmap id:", d)
	return
}
