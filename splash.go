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
	"image"
	"os"

	"code.google.com/p/graphics-go/graphics"
	"dlib/gio-2.0"
	"dlib/graphic"
	//"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/keybind"
	"github.com/BurntSushi/xgbutil/mousebind"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xprop"
	"github.com/BurntSushi/xgbutil/xwindow"
)

const (
	personalizationID          = "com.deepin.dde.personalization"
	gkeyCurrentBackground      = "current-picture"
	deepinBackgroundWindowProp = "DEEPIN_BACKGROUND_WINDOW"
	defaultBackgroundFile      = "/usr/share/backgrounds/default_background.jpg"
)

var (
	personSettings = gio.NewSettings(personalizationID)
	bgwin          *xwindow.Window
)

func drawBackground() {
	XU, err := xgbutil.NewConn()
	if err != nil {
		Logger.Error("could not create a new XUtil: ", err)
		return
	}

	// load background
	img, err := graphic.LoadImage(getBackgroundFile())
	if err != nil {
		Logger.Error("load background failed: ", err)
		return
	}

	// TODO fix background, clip and resize
	// screenWidth, screenHeight := 1024, 768
	screenWidth, screenHeight := getPrimaryScreenBestResolution()
	// fixedimg := graphic.ImplFillImage(img, int(screenWidth), int(screenHeight), graphic.FillScaleStretch)
	x0, y0, x1, y1 := graphic.GetScaleRectInImage(int(screenWidth), int(screenHeight),
		img.Bounds().Max.X, img.Bounds().Max.Y)
	clipimg := graphic.ImplClipImage(img, x0, y0, x1, y1)
	fixedimg := image.NewRGBA(image.Rect(0, 0, int(screenWidth), int(screenHeight)))
	graphics.Scale(fixedimg, clipimg)

	// convert it into an XU image.
	ximg := xgraphics.NewConvert(XU, fixedimg)

	// show it in a new window
	if bgwin != nil {
		showImage(bgwin, ximg)
	}
}

func createBgWindow(title string, quit bool) *xwindow.Window {
	XU, err := xgbutil.NewConn()
	if err != nil {
		Logger.Error("could not create a new XUtil: ", err)
		return nil
	}
	win, err := xwindow.Generate(XU)
	if err != nil {
		Logger.Error("could not generate new window id: ", err)
		return nil
	}
	win.Create(XU.RootWin(), 0, 0, 100, 100, 0)

	// make this window close gracefully
	win.WMGracefulClose(func(w *xwindow.Window) {
		xevent.Detach(w.X, w.Id)
		keybind.Detach(w.X, w.Id)
		mousebind.Detach(w.X, w.Id)
		w.Destroy()

		if quit {
			xevent.Quit(w.X)
		}
	})

	// set _NET_WM_NAME so it looks nice
	err = ewmh.WmNameSet(XU, win.Id, title)
	if err != nil {
		// not a fatal error
		Logger.Error("Could not set _NET_WM_NAME: ", err)
	}

	// set _NET_WM_WINDOW_TYPE_DESKTOP window type
	ewmh.WmWindowTypeSet(XU, win.Id, []string{"_NET_WM_WINDOW_TYPE_DESKTOP"})

	// set root window property
	xprop.ChangeProp32(XU, XU.RootWin(), deepinBackgroundWindowProp, "WINDOW", uint(win.Id))

	Logger.Info("background window id: ", win.Id)
	return win
}

// This is a slightly modified version of xgraphics.XShowExtra that does
// not set any resize constraints on the window (so that it can go
// fullscreen).
func showImage(win *xwindow.Window, im *xgraphics.Image) {
	// resize window with dimensions equal to the image
	w, h := im.Rect.Dx(), im.Rect.Dy()
	win.MoveResize(0, 0, w, h)

	// paint our image before mapping
	im.XSurfaceSet(win.Id)
	im.XDraw()
	im.XPaint(win.Id)

	// now we can map, since we've set all our properties.
	// (The initial map is when the window manager starts managing.)
	win.Map()
}

func getBackgroundFile() string {
	uri := personSettings.GetString(gkeyCurrentBackground)
	Logger.Debug("background uri: ", uri)
	path, ok, err := utils.URIToPath(uri)
	if !ok || !isFileExists(path) {
		Logger.Warning("get background file failed: ", err)
		Logger.Warning("use default background: ", defaultBackgroundFile)
		return defaultBackgroundFile
	}
	return path
}

func isFileExists(file string) bool {
	if _, err := os.Stat(file); err == nil {
		return true
	}
	return false
}

func listenBackgroundChanged() {
	personSettings.Connect("changed", func(s *gio.Settings, key string) {
		switch key {
		case gkeyCurrentBackground:
			Logger.Debug("background value in gsettings changed: ", key)
			drawBackground()
		}
	})
}

// TODO Get all screen's best resolution and choose a smaller one for there
// is no screen is primary.
func getPrimaryScreenBestResolution() (w uint16, h uint16) {
	w, h = 1024, 768 // default value

	_, err := randr.QueryVersion(X, 1, 4).Reply()
	if err != nil {
		Logger.Error("query randr failed: ", err)
		return
	}
	Root := xproto.Setup(X).DefaultScreen(X).Root
	resources, err := randr.GetScreenResources(X, Root).Reply()
	if err != nil {
		return
	}

	bestModes := make([]uint32, 0)
	for _, output := range resources.Outputs {
		reply, err := randr.GetOutputInfo(X, output, 0).Reply()
		if err == nil && reply.NumModes > 1 {
			bestModes = append(bestModes, uint32(reply.Modes[0]))
		}
	}

	w, h = 0, 0
	for _, m := range resources.Modes {
		for _, id := range bestModes {
			if id == m.Id {
				bw, bh := m.Width, m.Height
				if w*h == 0 {
					w, h = bw, bh
				} else if bw*bh > w*h {
					w, h = bw, bh
				}
			}
		}
	}

	Logger.Debugf("primary screen's best resolution is %dx%d", w, h)
	return
}
