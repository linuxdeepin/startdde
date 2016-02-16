/**
 * Copyright (C) 2014 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package display

import fmtp "github.com/kr/pretty"
import "testing"
import . "launchpad.net/gocheck"
import "github.com/BurntSushi/xgb/randr"
import "github.com/BurntSushi/xgb"

func Test(t *testing.T) { TestingT(t) }

func init() {
	X, _ := xgb.NewConn()
	randr.Init(X)
	randr.QueryVersion(X, 1, 3).Reply()
	Suite(GetDisplay())
}

func (dpy *Display) TestInfo(c *C) {
	/*fmtp.Println("DPY:", dpy)*/
}

func (m *Monitor) TestInfo(c *C) {
	fmtp.Println("Monitor:", m)
}
