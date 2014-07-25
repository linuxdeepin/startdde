package display

import . "launchpad.net/gocheck"
import "github.com/BurntSushi/xgb/xproto"

func (dpy *Display) TestIsOverlay(c *C) {
	c.Check(isOverlap(0, 0, 1440, 900, 0, 0, 1440, 900), Equals, true)
	c.Check(isOverlap(1, 1, 1440, 900, 0, 0, 1440, 900), Equals, true)
	c.Check(isOverlap(1, 1, 1440, 900, 1280, 0, 1440, 900), Equals, true)
	c.Check(isOverlap(1, 1, 1440, 900, 0, 800, 1440, 900), Equals, true)

	c.Check(isOverlap(0, 0, 1280, 800, 1280, 0, 1440, 900), Equals, false)
	c.Check(isOverlap(100, 100, 1440, 900, 1680, 0, 1440, 900), Equals, false)
}
func (dpy *Display) TestRunCode(c *C) {
	c.Check(runCode("ls"), Equals, true)
	c.Check(runCode("IThinkNoOneCanFindMe"), Equals, false)

	c.Check(runCodeAsync("ls"), Equals, true)
	//c.Check(runCodeAsync("IThinkNoOneCanFindMe"), Equals, false)
}
func (dpy *Display) TestAtom(c *C) {
	r := getAtom(xcon, "deepin")
	c.Check(r, Not(Equals), xproto.AtomNone)
	c.Check(queryAtomName(xcon, r), Equals, "deepin")
}

func (dpy *Display) isCrtcConnected(c *C) {
	c.Check(isCrtcConnected(xcon, 0), Equals, false)
}
