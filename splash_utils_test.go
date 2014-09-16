/**
 * Copyright (c) 2014 Deepin, Inc.
 *               2014 Xu FaSheng
 *
 * Author:      Xu FaSheng <fasheng.xu@gmail.com>
 * Maintainer:  Xu FaSheng <fasheng.xu@gmail.com>
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, see <http://www.gnu.org/licenses/>.
 **/

package main

import (
	C "launchpad.net/gocheck"
	"testing"
)

type splashTester struct{}

func TestT(t *testing.T) { C.TestingT(t) }

func init() {
	C.Suite(&splashTester{})
}

func delta(x, y float64) float64 {
	if x >= y {
		return x - y
	}
	return y - x
}
func similarEqual(x, y float64) bool {
	if delta(x, y) < 0.001 {
		return true
	}
	return false
}

func (*splashTester) TestSplashRenderFloat32ToFixed(c *C.C) {
	data := []struct {
		v float32
	}{
		{0},
		{1.0},
		{0.1},
		{0.2},
		{0.5},
		{0.7},
		{0.9},
		{1.1},
		{1.9},
		{5.0},
	}
	for _, d := range data {
		fixed := renderFloat32ToFixed(d.v)
		value := renderFixedToFloat32(fixed)
		c.Check(similarEqual(float64(value), float64(d.v)), C.Equals, true)
	}
}

func (*splashTester) TestGetClipRect(c *C.C) {
	_, useFullImage, _ := getClipRect(1024, 768, 1024, 768)
	c.Check(useFullImage, C.Equals, true)
	_, useFullImage, _ = getClipRect(512, 384, 1024, 768)
	c.Check(useFullImage, C.Equals, true)
	_, useFullImage, _ = getClipRect(1024, 768, 512, 384)
	c.Check(useFullImage, C.Equals, true)
	_, useFullImage, _ = getClipRect(1024, 768, 500, 500)
	c.Check(useFullImage, C.Equals, false)
}
