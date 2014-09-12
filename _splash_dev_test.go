package main

import (
	"fmt"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xprop"
	C "launchpad.net/gocheck"
	"pkg.linuxdeepin.com/lib/glib-2.0"
	"testing"
	"time"
)

type splashTester struct{}

func TestT(t *testing.T) { C.TestingT(t) }

func init() {
	C.Suite(&splashTester{})
}

func (*splashTester) TestSplash(c *C.C) {
	initSplash()
	initSplashAfterDependsLoaded()
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
	ewmh.WmWindowTypeSet(XU, win.Id, []string{"_NET_WM_WINDOW_TYPE_DESKTOP"})
	return true
}
func getRootProp(prop string) (d xproto.Drawable) {
	reply, _ := xprop.GetProperty(XU, XU.RootWin(), prop)
	d = xproto.Drawable(xgb.Get32(reply.Value))
	fmt.Println("pixmap id:", d)
	return
}
