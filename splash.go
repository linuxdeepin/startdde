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
	"sync"
	"time"

	"dbus/com/deepin/daemon/display"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/render"
	"github.com/BurntSushi/xgb/shape"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/xprop"
	"github.com/BurntSushi/xgbutil/xwindow"
	graphic "pkg.linuxdeepin.com/lib/gdkpixbuf"
	"pkg.linuxdeepin.com/lib/gio-2.0"
	"runtime"
	dd "runtime/debug"
)

const (
	personalizationID     = "com.deepin.dde.personalization"
	gkeyCurrentBackground = "picture-uri"
	defaultBackgroundFile = "/usr/share/backgrounds/default_background.jpg"

	ddeBgWindowTitle    = "DDE Background"
	ddeBgWindowProp     = "_DDE_BACKGROUND_WINDOW"
	ddeBgPixmapBlurProp = "_DDE_BACKGROUND_PIXMAP_BLURRED" // primary screen's background with blurred effect
)

var (
	XU, _       = xgbutil.NewConn()
	Display, _  = display.NewDisplay("com.deepin.daemon.Display", "/com/deepin/daemon/Display")
	bgGSettings = gio.NewSettings(personalizationID)
)

var (
	picFormat24 render.Pictformat
	picFormat32 render.Pictformat
)

var (
	filterNearest     xproto.Str
	filterBilinear    xproto.Str
	filterBest        xproto.Str
	filterConvolution xproto.Str
)

type crtcInfo struct {
	x      int16
	y      int16
	width  uint16
	height uint16
}

// union structure
var bgWinInfo = struct {
	win *xwindow.Window
	pid render.Picture
}{}
var bgImgInfo = struct {
	width  uint16
	height uint16
	pixmap xproto.Pixmap
	pid    render.Picture
	lock   sync.Mutex
}{}
var rootBgImgInfo = struct {
	bgPixmap     xproto.Pixmap
	bgBlurPixmap xproto.Pixmap
	lock         sync.Mutex
}{}

func initSplash() {
	graphic.InitGdkXlib()
	randr.Init(XU.Conn())
	randr.QueryVersion(XU.Conn(), 1, 4)
	render.Init(XU.Conn())
	render.QueryVersion(XU.Conn(), 0, 11)
	shape.Init(XU.Conn())

	// initialize structure
	bgWinInfo.pid, _ = render.NewPictureId(XU.Conn())
	bgImgInfo.pid, _ = render.NewPictureId(XU.Conn())

	bgWinInfo.win = createBgWindow(ddeBgWindowTitle)
	queryRender(xproto.Drawable(bgWinInfo.win.Id))

	// bind picture id to background window
	err := render.CreatePictureChecked(XU.Conn(), bgWinInfo.pid, xproto.Drawable(bgWinInfo.win.Id), picFormat24, 0, nil).Check()
	if err != nil {
		logger.Error("create render picture failed:", err)
	}
}

func initSplashAfterDependsLoaded() {
	loadBgFile()

	// TODO: notifyRedrawBgWindow()
	// background will be draw after receiving window expose
	// event, but here we update it again after a few time to
	// solve draw fails problem when compiz not ready
	drawBg()
	go func() {
		time.Sleep(time.Second * 5)
		drawBg()
	}()
	mapBgToRoot()

	listenBgFileChanged()
	go listenDisplayChanged()
}

func queryRender(d xproto.Drawable) {
	// get picture formats
	formats, _ := render.QueryPictFormats(XU.Conn()).Reply()
	for _, f := range formats.Formats {
		if f.Depth == 24 && f.Type == render.PictTypeDirect {
			picFormat24 = f.Id
		} else if f.Depth == 32 && f.Type == render.PictTypeDirect {
			picFormat32 = f.Id
		}
	}
	logger.Debugf("picture format32=%d, format24=%d", picFormat32, picFormat24)

	// get image filters
	filters, _ := render.QueryFilters(XU.Conn(), d).Reply()
	for _, f := range filters.Filters {
		switch f.Name {
		case "nearest":
			filterNearest = f
		case "bilinear":
			filterBilinear = f
		case "best":
			filterBest = f
		case "convolution":
			filterConvolution = f
		}
	}
	logger.Debug("render filter:", filters.Filters)
}

func createBgWindow(title string) *xwindow.Window {
	win, err := xwindow.Generate(XU)
	if err != nil {
		logger.Error("could not generate new window id:", err)
		return nil
	}
	w, h := getScreenResolution()
	win.Create(XU.RootWin(), 0, 0, int(w), int(h), 0)

	// set _NET_WM_NAME so it looks nice
	err = ewmh.WmNameSet(XU, win.Id, title)
	if err != nil {
		logger.Error(err) // not a fatal error
	}

	// set _NET_WM_WINDOW_TYPE_DESKTOP window type
	ewmh.WmWindowTypeSet(XU, win.Id, []string{"_NET_WM_WINDOW_TYPE_DESKTOP"})

	// disable input event
	err = shape.RectanglesChecked(XU.Conn(), shape.SoSet, shape.SkInput, 0, win.Id, 0, 0, nil).Check()
	if err != nil {
		logger.Error(err) // not a fatal error
	}

	// map window
	// TODO: remove such code when deepin-wm is completed
	switch *WindowManager {
	case "compiz":
		win.Map()
	case "deepin":
		// do nothing
	}

	// create property on root window
	err = xprop.ChangeProp32(XU, XU.RootWin(), ddeBgWindowProp, "WINDOW", uint(win.Id))
	if err != nil {
		logger.Error(err) // not a fatal error
	}

	logger.Info("background window id:", win.Id)
	return win
}

func loadBgFile() {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("loadBgFile failed:", err)
		}
	}()

	bgImgInfo.lock.Lock()
	defer bgImgInfo.lock.Unlock()

	// load background file and convert into XU image
	bgfile := getBackgroundFile()
	w, h, err := graphic.GetImageSize(bgfile)
	if err != nil {
		logger.Error(err)
		return
	}
	bgImgInfo.width = uint16(w)
	bgImgInfo.height = uint16(h)

	// rebind picture id to background pixmap
	xproto.FreePixmap(XU.Conn(), bgImgInfo.pixmap)
	bgImgInfo.pixmap, err = convertImageToXpixmap(bgfile)
	if err != nil {
		logger.Error(err)
		return
	}
	logger.Debugf("bgImgInfo %v", bgImgInfo)
	render.FreePicture(XU.Conn(), bgImgInfo.pid)
	err = render.CreatePictureChecked(XU.Conn(), bgImgInfo.pid, xproto.Drawable(bgImgInfo.pixmap), picFormat24, 0, nil).Check()
	if err != nil {
		logger.Error("create render picture failed:", err)
		return
	}

	// setup image filter
	err = render.SetPictureFilterChecked(XU.Conn(), bgImgInfo.pid, uint16(filterBilinear.NameLen), filterBilinear.Name, nil).Check()
	if err != nil {
		logger.Error("set picture filter failed:", err)
	}

	runtime.GC()
}

func drawBg() {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("drawBg failed", err)
		}
	}()

	resources, err := randr.GetScreenResources(XU.Conn(), XU.RootWin()).Reply()
	if err != nil {
		logger.Error("get scrren resources failed:", err)
		return
	}

	for _, output := range resources.Outputs {
		reply, err := randr.GetOutputInfo(XU.Conn(), output, 0).Reply()
		if err != nil {
			logger.Warning("get output info failed:", err)
			continue
		}
		if reply.Connection != randr.ConnectionConnected {
			continue
		}
		cinfo, err := randr.GetCrtcInfo(XU.Conn(), reply.Crtc, 0).Reply()
		if err != nil {
			logger.Warningf("get crtc info failed: id %d, %v", reply.Crtc, err)
			continue
		}
		doDrawBgByRender(bgImgInfo.pid, bgWinInfo.pid, cinfo.X, cinfo.Y, cinfo.Width, cinfo.Height)
	}
}

func doDrawBgByRender(srcpid, dstpid render.Picture, x, y int16, width, height uint16) {
	logger.Infof("doDrawBgByRender: x=%d, y=%d, width=%d, height=%d", x, y, width, height)

	defer func() {
		if err := recover(); err != nil {
			logger.Error("doDrawBgByRender() failed:", err)
		}
	}()

	if width <= 0 || height <= 0 {
		logger.Warning("width or height invalid:", width, height)
		return
	}

	// get clip rectangle
	rect, _, err := getClipRect(width, height, getBgImgWidth(), getBgImgHeight())
	if err != nil {
		logger.Error(err)
		return
	}
	logger.Debug("doDrawBgByRender: clip rect", rect)

	// scale source image and clip rectangle

	// FIXME: to ignore the drawing issue caused by
	// gdkpixbuf/convertImageToXpixmap() that there will be a
	// horizontal line in the bottom right corner if the size of
	// background image and screen is similar, we increase the scale
	// value a little here
	s := float32(height+10) / float32(rect.Height)

	rect.X = int16(float32(rect.X) * s)
	rect.Y = int16(float32(rect.Y) * s)
	rect.Width = uint16(float32(rect.Width) * s)
	rect.Height = uint16(float32(rect.Height) * s)
	logger.Debug("doDrawBgByRender: scaled clip rect", rect)

	t := renderGetScaleTransform(s, s)
	logger.Debugf("doDrawBgByRender: scale transform, s=%f, %x", s, t)
	err = render.SetPictureTransformChecked(XU.Conn(), srcpid, t).Check()
	if err != nil {
		logger.Error(err)
		return
	}

	// draw to background window
	err = render.CompositeChecked(XU.Conn(), render.PictOpSrc, srcpid, 0, dstpid,
		rect.X, rect.Y, 0, 0, x, y, width, height).Check()
	if err != nil {
		logger.Error(err)
		return
	}

	// restore source image
	t = renderGetScaleTransform(1/s, 1/s)
	logger.Debugf("doDrawBgByRender: restore scale transform, s=%f, s=%f, %x", 1/s, 1/s, t)
	err = render.SetPictureTransformChecked(XU.Conn(), srcpid, t).Check()
	if err != nil {
		logger.Error(err)
		return
	}
}

func mapBgToRoot() {
	defer func() {
		if err := recover(); err != nil {
			// error occurred if background file is busy for that user
			// change background frequently
			logger.Error(err)
		}
	}()

	// generate temporary background file, same size with primary screen
	w, h := getPrimaryScreenResolution()
	logger.Debug("mapBgToRoot() screen resolution:", w, h)
	rootBgFile, _, err := graphic.ScaleImagePreferCache(getBackgroundFile(), int(w), int(h),
		graphic.GDK_INTERP_BILINEAR, graphic.FormatPng)
	if err != nil {
		logger.Error(err)
		return
	}
	logger.Debug("mapBgToRoot() generate rootBgFile success")

	// generate temporary blurred background file
	rootBgBlurFile, _, err := graphic.BlurImageCache(rootBgFile, 50, 1, graphic.FormatPng)
	if err != nil {
		logger.Error(err)
		return
	}
	logger.Debug("mapBgToRoot() generate rootBgBlurFile success")

	// set root window properties
	doMapBgToRoot(rootBgBlurFile)
}

func doMapBgToRoot(rootBgBlurFile string) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error(err)
		}
	}()

	rootBgImgInfo.lock.Lock()
	defer rootBgImgInfo.lock.Unlock()

	var err error
	xproto.FreePixmap(XU.Conn(), rootBgImgInfo.bgBlurPixmap)
	rootBgImgInfo.bgBlurPixmap, err = xcbConvertImageToXpixmap(rootBgBlurFile)
	if err != nil {
		logger.Error(err)
		return
	}
	err = xprop.ChangeProp32(XU, XU.RootWin(), ddeBgPixmapBlurProp, "PIXMAP", uint(rootBgImgInfo.bgBlurPixmap))
	if err != nil {
		logger.Error(err)
		return
	}

	// quickly free ximage used memory
	dd.FreeOSMemory()
}

func resizeBgWindow(w, h int) {
	logger.Debugf("background resizing: %dx%d", w, h)
	bgWinInfo.win.MoveResize(0, 0, w, h)
}

func listenBgFileChanged() {
	bgGSettings.Connect("changed", func(s *gio.Settings, key string) {
		switch key {
		case gkeyCurrentBackground:
			logger.Info("background changed:", key, getBackgroundFile())
			loadBgFile()
			drawBg()
			mapBgToRoot()
		}
	})
}

func listenDisplayChanged() {
	Display.PrimaryRect.ConnectChanged(func() {
		logger.Info("Display.PrimaryRect changed:", Display.PrimaryRect.Get())
		mapBgToRoot()
	})

	bgWinInfo.win.Listen(xproto.EventMaskExposure)
	randr.SelectInput(XU.Conn(), XU.RootWin(), randr.NotifyMaskCrtcChange|randr.NotifyMaskScreenChange)
	for {
		event, err := XU.Conn().WaitForEvent()
		if err != nil {
			continue
		}
		switch e := event.(type) {
		case xproto.ExposeEvent:
			logger.Info("expose event:", e)
			if e.Count == 0 {
				// if count is zero, no more Expose events for this
				// window follow, so we draw background in this case
				drawBg()
			}
		case randr.ScreenChangeNotifyEvent:
			logger.Infof("ScreenChangeNotifyEvent: %dx%d", e.Width, e.Height)

			// skip invalid event for window manager issue
			if e.Width < 640 && e.Height < 480 {
				continue
			}

			if e.Rotation == randr.RotationRotate90 || e.Rotation == randr.RotationRotate270 {
				e.Width, e.Height = e.Height, e.Width
			}
			resizeBgWindow(int(e.Width), int(e.Height))

			// TODO: notifyRedrawBgWindow()
			// send expose event to self manually to fix the first
			// monitor's background drawing issue if exists multiple
			// monitors.
			event := xproto.ExposeEvent{
				Window: bgWinInfo.win.Id,
			}
			xproto.SendEvent(XU.Conn(), false, bgWinInfo.win.Id, xproto.EventMaskExposure, string(event.Bytes()))
		}
	}
}
