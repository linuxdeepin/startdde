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
	_ "image/jpeg"
	_ "image/png"
	"sync"
	"time"

	"dbus/com/deepin/daemon/display"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/render"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/xprop"
	"github.com/BurntSushi/xgbutil/xwindow"
	"pkg.linuxdeepin.com/lib/gio-2.0"
	"pkg.linuxdeepin.com/lib/graphic"
	"runtime"
)

const (
	personalizationID     = "com.deepin.dde.personalization"
	gkeyCurrentBackground = "current-picture"
	ddeBgPixmapProp       = "_DDE_BACKGROUND_PIXMAP"
	ddeBgPixmapBlurProp   = "_DDE_BACKGROUND_PIXMAP_BLURRED"
	ddeBgWindowTitle      = "DDE Background"
	defaultBackgroundFile = "/usr/share/backgrounds/default_background.jpg"
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
	pid    render.Picture
	lock   sync.Mutex
}{}
var rootBgImgInfo = struct {
	// TODO refactor code
	lock sync.Mutex
}{}

func initBackground() {
	initGdkXlib()
	randr.Init(XU.Conn())
	randr.QueryVersion(XU.Conn(), 1, 4)
	render.Init(XU.Conn())
	render.QueryVersion(XU.Conn(), 0, 11)

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

func initBackgroundAfterDependsLoaded() {
	loadBgFile()

	// background will be draw after receiving window expose
	// event, but here we update it again after a few time to
	// solve draw fails problem when compiz not ready
	drawBackground()
	go func() {
		time.Sleep(time.Second * 5)
		drawBackground()
	}()

	mapBgToRoot()
	Display.PrimaryRect.ConnectChanged(func() {
		mapBgToRoot()
	})

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
		// not a fatal error
		logger.Error("Could not set _NET_WM_NAME:", err)
	}

	// set _NET_WM_WINDOW_TYPE_DESKTOP window type
	ewmh.WmWindowTypeSet(XU, win.Id, []string{"_NET_WM_WINDOW_TYPE_DESKTOP"})

	win.Map()
	return win
}

func loadBgFile() {
	logger.Debug("loadBgFile() begin")
	defer logger.Debug("loadBgFile() end")

	// TODO remove defer
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
	pixmap, err := convertToXpixmap(bgfile)
	if err != nil {
		panic(err)
	}
	logger.Debugf("bgImgInfo.pid=%d, bgWinInfo.pid=%d, pixmap=%d", bgImgInfo.pid, bgWinInfo.pid, pixmap)
	render.FreePicture(XU.Conn(), bgImgInfo.pid)
	err = render.CreatePictureChecked(XU.Conn(), bgImgInfo.pid, xproto.Drawable(pixmap), picFormat24, 0, nil).Check()
	if err != nil {
		logger.Error("create render picture failed:", err)
		return
	}

	// setup image filter
	err = render.SetPictureFilterChecked(XU.Conn(), bgImgInfo.pid, uint16(filterBilinear.NameLen), filterBilinear.Name, nil).Check()
	// TODO: choose a better filter
	// err = render.SetPictureFilterChecked(XU.Conn(), bgImgInfo.pid, uint16(filterNearest.NameLen), filterNearest.Name, nil).Check()
	// err = render.SetPictureFilterChecked(XU.Conn(), bgImgInfo.pid, uint16(filterBest.NameLen), filterBest.Name, nil).Check()
	// TODO: test only, for convolution filter
	// err = render.SetPictureFilterChecked(XU.Conn(), bgImgInfo.pid, uint16(filterConvolution.NameLen), filterConvolution.Name, []render.Fixed{2621440, 2621440}).Check()
	// err = render.SetPictureFilterChecked(XU.Conn(), bgImgInfo.pid, 1602*uint16(filterConvolution.NameLen), filterConvolution.Name, []render.Fixed{2621440, 2621440}).Check()
	if err != nil {
		logger.Error("set picture filter failed:", err)
	}

	runtime.GC() // TODO
}

func drawBackground() {
	logger.Info("drawBackground() begin")
	defer logger.Info("drawBackground() end")

	defer func() {
		if err := recover(); err != nil {
			logger.Error("drawBackground failed", err)
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
	defer func() {
		if err := recover(); err != nil {
			logger.Error("doDrawBgByRender() failed:", err)
		}
	}()

	logger.Debugf("draw background through xrender: x=%d, y=%d, width=%d, height=%d", x, y, width, height)

	// get clip rectangle
	rect, err := getClipRect(width, height, getBgImgWidth(), getBgImgHeight())
	if err != nil {
		panic(err)
	}
	logger.Debug("drawBgByRender, clip rect", rect)

	// scale source image and clip rectangle
	sx := float32(width) / float32(rect.Width)
	sy := float32(height) / float32(rect.Height)
	rect.X = int16(float32(rect.X) * sx)
	rect.Y = int16(float32(rect.Y) * sy)
	rect.Width = uint16(float32(rect.Width) * sx)
	rect.Height = uint16(float32(rect.Height) * sx)
	t := getScaleTransform(sx, sy)
	logger.Debugf("scale transform: sx=%f, sy=%f, %x", sx, sy, t)
	err = render.SetPictureTransformChecked(XU.Conn(), srcpid, t).Check()
	if err != nil {
		panic(err)
	}

	// draw to background window
	err = render.CompositeChecked(XU.Conn(), render.PictOpSrc, srcpid, 0, dstpid,
		rect.X, rect.Y, 0, 0, x, y, width, height).Check()
	if err != nil {
		panic(err)
	}

	// restore source image
	t = getScaleTransform(1/sx, 1/sy)
	err = render.SetPictureTransformChecked(XU.Conn(), srcpid, t).Check()
	if err != nil {
		panic(err)
	}
}

func mapBgToRoot() {
	logger.Info("mapBgToRoot() begin")
	defer logger.Info("mapBgToRoot() end")

	defer func() {
		if err := recover(); err != nil {
			// error occurred if background file is busy for user
			// change background frequent
			logger.Error(err)
		}
	}()

	// generate temporary background file, same size with primary screen
	w, h := getPrimaryScreenResolution()
	logger.Debug("mapBgToRoot() screen resolution:", w, h)
	rootBgFile, useCacheRootBg, err := graphic.FillImageCache(getBackgroundFile(), int(w), int(h),
		graphic.FillProportionCenterScale, graphic.PNG)
	if err != nil {
		panic(err)
	}
	logger.Debug("mapBgToRoot() generate rootBgFile end")

	// generate temporary blurred background file
	rootBgBlurFile, useCacheRootBgBlue, err := graphic.BlurImageCache(rootBgFile, 50, 1, graphic.PNG)
	if err != nil {
		panic(err)
	}
	logger.Debug("mapBgToRoot() generate rootBgBlurFile end")

	// set root window properties
	doMapBgToRoot(rootBgFile, rootBgBlurFile)

	// TODO
	// if use cache file, keep it update to time
	if useCacheRootBg || useCacheRootBgBlue {
		// 	if err := graphic.FillImage(getBackgroundFile(), rootBgFile, int(w), int(h),
		// 		graphic.FillProportionCenterScale, graphic.PNG); err != nil {
		// 		if err := graphic.BlurImage(rootBgFile, rootBgBlurFile, 50, 1, graphic.PNG); err != nil {
		// 			// set root window properties again
		// 			doMapBgToRoot(rootBgFile, rootBgBlurFile)
		// 		}
		// 	}
	}
}
func doMapBgToRoot(rootBgFile, rootBgBlurFile string) {
	logger.Debug("doMapBgToRoot() begin")
	defer logger.Debug("doMapBgToRoot() end")

	defer func() {
		if err := recover(); err != nil {
			logger.Error(err)
		}
	}()

	rootBgImgInfo.lock.Lock()
	defer rootBgImgInfo.lock.Unlock()

	bgPixmap, err := convertToXpixmap(rootBgFile)
	if err != nil {
		panic(err)
	}
	err = xprop.ChangeProp32(XU, XU.RootWin(), ddeBgPixmapProp, "PIXMAP", uint(bgPixmap))
	if err != nil {
		panic(err)
	}

	bgBlurPixmap, err := convertToXpixmap(rootBgBlurFile)
	if err != nil {
		panic(err)
	}
	err = xprop.ChangeProp32(XU, XU.RootWin(), ddeBgPixmapBlurProp, "PIXMAP", uint(bgBlurPixmap))
	if err != nil {
		panic(err)
	}

	runtime.GC() // TODO
}

func resizeBgWindow(w, h int) {
	geom, _ := bgWinInfo.win.Geometry()
	if geom.Width() == w && geom.Height() == h {
		return
	}
	logger.Debugf("background window before resizing, %dx%d", geom.Width(), geom.Height())
	bgWinInfo.win.MoveResize(0, 0, w, h)
	geom, _ = bgWinInfo.win.Geometry()
	logger.Debugf("background window after resizing, %dx%d", geom.Width(), geom.Height())
	drawBackground()
}

func listenBgFileChanged() {
	r := NewOverrideRunner()
	go r.Loop()
	bgGSettings.Connect("changed", func(s *gio.Settings, key string) {
		switch key {
		case gkeyCurrentBackground:
			logger.Info("background value in gsettings changed:", key, getBackgroundFile())
			r.AddTaskGroup(
				loadBgFile,
				drawBackground,
				mapBgToRoot,
			)
		}
	})
}

func listenDisplayChanged() {
	bgWinInfo.win.Listen(xproto.EventMaskExposure)
	randr.SelectInput(XU.Conn(), XU.RootWin(), randr.NotifyMaskCrtcChange|randr.NotifyMaskScreenChange)
	for {
		event, err := XU.Conn().WaitForEvent()
		if err != nil {
			continue
		}
		switch e := event.(type) {
		case xproto.ExposeEvent:
			logger.Debug("expose event", e)
			drawBackground()
		case randr.ScreenChangeNotifyEvent:
			logger.Debugf("ScreenChangeNotifyEvent: %dx%d", e.Width, e.Height)

			// skip invalid event for window manager issue
			if e.Width < 640 && e.Height < 480 {
				continue
			}

			if e.Rotation == randr.RotationRotate90 || e.Rotation == randr.RotationRotate270 {
				e.Width, e.Height = e.Height, e.Width
			}
			resizeBgWindow(int(e.Width), int(e.Height))
		}
	}
}
