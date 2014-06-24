package main

import (
	"testing"
	// "time"
	// "pkg.linuxdeepin.com/lib/glib-2.0"
	"fmt"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xprop"
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

// TODO
// func TestSplash(t *testing.T) {
// 	initBackground()
// 	initBackgroundAfterDependsLoaded()
// 	go glib.StartLoop()
// 	// time.Sleep(10 * time.Second)
// 	select {}
// }

func TestReadRootPixmap(t *testing.T) {
	if drawWindowThroughRootPixmap() {
		select {}
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
