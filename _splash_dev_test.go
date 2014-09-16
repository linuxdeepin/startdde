package main

import (
	"fmt"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/shape"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xprop"
	C "launchpad.net/gocheck"
	"pkg.linuxdeepin.com/lib/glib-2.0"
	"time"
)

func (*splashTester) TestSplash(c *C.C) {
	initSplash()
	initSplashAfterDependsLoaded()

	// enable input event
	inputRect := make([]xproto.Rectangle, 1)
	winRect, _ := bgWinInfo.win.Geometry()
	inputRect[0] = xproto.Rectangle{X: 0, Y: 0, Width: uint16(winRect.Width()), Height: uint16(winRect.Height())}
	logger.Info(inputRect)
	err := shape.RectanglesChecked(XU.Conn(), shape.SoSet, shape.SkInput, 0, bgWinInfo.win.Id, 0, 0, inputRect).Check()
	if err != nil {
		logger.Error(err) // not a fatal error
	}

	go glib.StartLoop()
	time.Sleep(600 * time.Second)
}

func (*splashTester) TestSplashReadRootProp(c *C.C) {
	if drawWindowThroughRootProp(ddeBgWindowProp) {
		time.Sleep(300 * time.Second)
	}
}
func drawWindowThroughRootProp(prop string) bool {
	ximg, err := xgraphics.NewDrawable(XU, getRootProp(prop))
	if err != nil {
		fmt.Println("error:", err)
		return false
	}
	win := ximg.XShow()
	// make window fullscreen
	ewmh.WmWindowTypeSet(XU, win.Id, []string{"_NET_WM_WINDOW_TYPE_SPLASH"})
	// icccm.WmStateSet(XU, win.Id, []string{"_NET_WM_STATE_FULLSCREEN"})
	win.Move(0, 0)
	return true
}
func getRootProp(prop string) (d xproto.Drawable) {
	reply, _ := xprop.GetProperty(XU, XU.RootWin(), prop)
	d = xproto.Drawable(xgb.Get32(reply.Value))
	fmt.Println("pixmap id:", d)
	return
}
