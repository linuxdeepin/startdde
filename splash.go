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
	"dlib/gio-2.0"
	"dlib/graphic"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/render"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xprop"
	"github.com/BurntSushi/xgbutil/xwindow"
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
	XU, _          = xgbutil.NewConn()
	Display, _     = display.NewDisplay("com.deepin.daemon.Display", "/com/deepin/daemon/Display")
	_bgGSettings   = gio.NewSettings(personalizationID)
	_crtcInfos     = make(map[randr.Crtc]*crtcInfo) // TODO remove
	_crtcInfosLock = sync.Mutex{}
)

var (
	_picFormat24 render.Pictformat
	_picFormat32 render.Pictformat
)

var (
	_filterNearest     xproto.Str
	_filterBilinear    xproto.Str
	_filterBest        xproto.Str
	_filterConvolution xproto.Str
)

type crtcInfo struct {
	x      int16
	y      int16
	width  uint16
	height uint16
}

// union structure
var _bgWinInfo = struct {
	win *xwindow.Window
	pid render.Picture
}{}
var _bgImgInfo = struct {
	img  *xgraphics.Image
	pid  render.Picture
	lock sync.Mutex
}{}
var _rootBgImgInfo = struct {
	bgImg     *xgraphics.Image
	bgBlurImg *xgraphics.Image
	lock      sync.Mutex
}{}

func initBackground() {
	randr.Init(XU.Conn())
	randr.QueryVersion(XU.Conn(), 1, 4)
	render.Init(XU.Conn())
	render.QueryVersion(XU.Conn(), 0, 11)

	// initialize structure
	_bgWinInfo.pid, _ = render.NewPictureId(XU.Conn())
	_bgImgInfo.pid, _ = render.NewPictureId(XU.Conn())

	_bgWinInfo.win = createBgWindow(ddeBgWindowTitle)
	queryRender(xproto.Drawable(_bgWinInfo.win.Id))

	// bind picture id to background window
	err := render.CreatePictureChecked(XU.Conn(), _bgWinInfo.pid, xproto.Drawable(_bgWinInfo.win.Id), _picFormat24, 0, nil).Check()
	if err != nil {
		Logger.Error("create render picture failed:", err)
	}
}

func initBackgroundAfterDependsLoaded() {
	go func() {
		// when for display daemon loaded
		time.Sleep(500 * time.Millisecond)
		mapBgToRoot()
	}()

	loadBgFile()

	// background will be draw after receiving window expose
	// event, but here we update it again after a few time to
	// solve draw fails problem when compiz not ready
	drawBackground()
	go func() {
		time.Sleep(time.Second * 5)
		drawBackground()
	}()

	listenBgFileChanged()
	go listenDisplayChanged()
}

func queryRender(d xproto.Drawable) {
	// get picture formats
	formats, _ := render.QueryPictFormats(XU.Conn()).Reply()
	for _, f := range formats.Formats {
		if f.Depth == 24 && f.Type == render.PictTypeDirect {
			_picFormat24 = f.Id
		} else if f.Depth == 32 && f.Type == render.PictTypeDirect {
			_picFormat32 = f.Id
		}
	}
	Logger.Debugf("picture format32=%d, format24=%d", _picFormat32, _picFormat24)

	// get image filters
	filters, _ := render.QueryFilters(XU.Conn(), d).Reply()
	for _, f := range filters.Filters {
		switch f.Name {
		case "nearest":
			_filterNearest = f
		case "bilinear":
			_filterBilinear = f
		case "best":
			_filterBest = f
		case "convolution":
			_filterConvolution = f
		}
	}
	Logger.Debug("render filter:", filters.Filters)
}

func createBgWindow(title string) *xwindow.Window {
	win, err := xwindow.Generate(XU)
	if err != nil {
		Logger.Error("could not generate new window id:", err)
		return nil
	}
	w, h := getScreenResolution()
	win.Create(XU.RootWin(), 0, 0, int(w), int(h), 0)

	// set _NET_WM_NAME so it looks nice
	err = ewmh.WmNameSet(XU, win.Id, title)
	if err != nil {
		// not a fatal error
		Logger.Error("Could not set _NET_WM_NAME:", err)
	}

	// set _NET_WM_WINDOW_TYPE_DESKTOP window type
	ewmh.WmWindowTypeSet(XU, win.Id, []string{"_NET_WM_WINDOW_TYPE_DESKTOP"})

	win.Map()
	return win
}

func loadBgFile() {
	Logger.Debug("loadBgFile() begin")
	defer Logger.Debug("loadBgFile() end")

	defer func() {
		if err := recover(); err != nil {
			Logger.Error("loadBgFile failed:", err)
		}
	}()

	_bgImgInfo.lock.Lock()
	defer _bgImgInfo.lock.Unlock()

	// load background file and convert into XU image
	_bgImgInfo.img = convertToXimage(getBackgroundFile(), _bgImgInfo.img)

	// rebind picture id to background pixmap
	Logger.Debugf("_bgImgInfo.pid=%d, _bgWinInfo.pid=%d", _bgImgInfo.pid, _bgWinInfo.pid)
	render.FreePicture(XU.Conn(), _bgImgInfo.pid)
	err := render.CreatePictureChecked(XU.Conn(), _bgImgInfo.pid, xproto.Drawable(_bgImgInfo.img.Pixmap), _picFormat24, 0, nil).Check()
	if err != nil {
		Logger.Error("create render picture failed:", err)
		return
	}
	// setup image filter
	err = render.SetPictureFilterChecked(XU.Conn(), _bgImgInfo.pid, uint16(_filterBilinear.NameLen), _filterBilinear.Name, nil).Check()
	// TODO: choose a better fileter
	// err = render.SetPictureFilterChecked(XU.Conn(), _bgImgInfo.pid, uint16(_filterNearest.NameLen), _filterNearest.Name, nil).Check()
	// err = render.SetPictureFilterChecked(XU.Conn(), _bgImgInfo.pid, uint16(_filterBest.NameLen), _filterBest.Name, nil).Check()
	// TODO: test only, for convolution filter
	// err = render.SetPictureFilterChecked(XU.Conn(), _bgImgInfo.pid, uint16(_filterConvolution.NameLen), _filterConvolution.Name, []render.Fixed{2621440, 2621440}).Check()
	// err = render.SetPictureFilterChecked(XU.Conn(), _bgImgInfo.pid, 1602*uint16(_filterConvolution.NameLen), _filterConvolution.Name, []render.Fixed{2621440, 2621440}).Check()
	if err != nil {
		Logger.Error("set picture filter failed:", err)
	}
}

func drawBackground() {
	Logger.Debug("drawBackground() begin")
	defer Logger.Debug("drawBackground() end")

	resources, err := randr.GetScreenResources(XU.Conn(), XU.RootWin()).Reply()
	if err != nil {
		Logger.Error("get scrren resources failed:", err)
		return
	}

	for _, output := range resources.Outputs {
		reply, err := randr.GetOutputInfo(XU.Conn(), output, 0).Reply()
		if err != nil {
			Logger.Warning("get output info failed:", err)
			continue
		}
		if reply.Connection != randr.ConnectionConnected {
			continue
		}
		cinfo, err := randr.GetCrtcInfo(XU.Conn(), reply.Crtc, 0).Reply()
		if err != nil {
			Logger.Warningf("get crtc info failed: id %d, %v", reply.Crtc, err)
			continue
		}
		doDrawBgByRender(_bgImgInfo.pid, _bgWinInfo.pid, cinfo.X, cinfo.Y, cinfo.Width, cinfo.Height)
	}
}

func doDrawBgByRender(srcpid, dstpid render.Picture, x, y int16, width, height uint16) {
	defer func() {
		if err := recover(); err != nil {
			Logger.Error("doDrawBgByRender() failed:", err)
		}
	}()

	Logger.Debugf("draw background through xrender: x=%d, y=%d, width=%d, height=%d", x, y, width, height)

	// get clip rectangle
	rect, err := getClipRect(width, height, getBgImgWidth(), getBgImgHeight())
	if err != nil {
		panic(err)
	}
	Logger.Debug("drawBgByRender, clip rect", rect)

	// scale source image and clip rectangle
	sx := float32(width) / float32(rect.Width)
	sy := float32(height) / float32(rect.Height)
	rect.X = int16(float32(rect.X) * sx)
	rect.Y = int16(float32(rect.Y) * sy)
	rect.Width = uint16(float32(rect.Width) * sx)
	rect.Height = uint16(float32(rect.Height) * sx)
	t := getScaleTransform(sx, sy)
	Logger.Debugf("scale transform: sx=%f, sy=%f, %x", sx, sy, t)
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

// TODO [re-implemented through xrender]
// TODO cancel operate if background file changed
func mapBgToRoot() {
	Logger.Info("mapBgToRoot() begin")
	defer Logger.Info("mapBgToRoot() end")

	defer func() {
		if err := recover(); err != nil {
			// error occurred if background file is busy for user
			// change background frequent
			Logger.Error(err)
		}
	}()

	// generate temporary background file, same size with primary screen
	w, h := getPrimaryScreenResolution()
	Logger.Info("mapBgToRoot() screen resolution:", w, h)
	rootBgFile, useCacheRootBg, err := graphic.FillImageCache(getBackgroundFile(), int32(w), int32(h),
		graphic.FillProportionCenterScale, graphic.PNG)
	if err != nil {
		panic(err)
	}
	Logger.Info("mapBgToRoot() generate rootBgFile end")

	// generate temporary blurred background file
	rootBgBlurFile, useCacheRootBgBlue, err := graphic.BlurImageCache(rootBgFile, 50, 1, graphic.PNG)
	if err != nil {
		panic(err)
	}
	Logger.Info("mapBgToRoot() generate rootBgBlurFile end")

	// set root window properties
	doMapBgToRoot(rootBgFile, rootBgBlurFile)

	// if use cache file, keep it update to time
	if useCacheRootBg || useCacheRootBgBlue {
		if err := graphic.FillImage(getBackgroundFile(), rootBgFile, int32(w), int32(h),
			graphic.FillProportionCenterScale, graphic.PNG); err != nil {
			if err := graphic.BlurImage(rootBgFile, rootBgBlurFile, 50, 1, graphic.PNG); err != nil {
				// set root window properties again
				doMapBgToRoot(rootBgFile, rootBgBlurFile)
			}
		}
	}
}
func doMapBgToRoot(rootBgFile, rootBgBlurFile string) {
	Logger.Info("doMapBgToRoot() begin")
	defer Logger.Info("doMapBgToRoot() end")

	defer func() {
		if err := recover(); err != nil {
			Logger.Error(err)
		}
	}()

	_rootBgImgInfo.lock.Lock()
	Logger.Info("doMapBgToRoot() lock _rootBgImgInfo")
	defer func() {
		// TODO test
		_rootBgImgInfo.lock.Unlock()
		Logger.Info("doMapBgToRoot() unlock _rootBgImgInfo")
	}()

	_rootBgImgInfo.bgImg = convertToXimage(rootBgFile, _rootBgImgInfo.bgImg)
	err := xprop.ChangeProp32(XU, XU.RootWin(), ddeBgPixmapProp, "PIXMAP", uint(_rootBgImgInfo.bgImg.Pixmap))
	if err != nil {
		panic(err)
	}
	Logger.Info("doMapBgToRoot() root window property: rootBgImg", uint(_rootBgImgInfo.bgImg.Pixmap))

	_rootBgImgInfo.bgBlurImg = convertToXimage(rootBgBlurFile, _rootBgImgInfo.bgBlurImg)
	err = xprop.ChangeProp32(XU, XU.RootWin(), ddeBgPixmapBlurProp, "PIXMAP", uint(_rootBgImgInfo.bgBlurImg.Pixmap))
	if err != nil {
		panic(err)
	}
	Logger.Info("doMapBgToRoot() root window property: rootBgBlurImg", uint(_rootBgImgInfo.bgBlurImg.Pixmap))
}

func resizeBgWindow(w, h int) {
	geom, _ := _bgWinInfo.win.Geometry()
	if geom.Width() == w && geom.Height() == h {
		return
	}
	Logger.Debugf("background window before resizing, %dx%d", geom.Width(), geom.Height())
	_bgWinInfo.win.MoveResize(0, 0, w, h)
	geom, _ = _bgWinInfo.win.Geometry()
	Logger.Debugf("background window after resizing, %dx%d", geom.Width(), geom.Height())
	drawBackground()
}

func updateCrtcInfos(crtc randr.Crtc, x, y int16, width, height uint16) (needRedraw bool) {
	_crtcInfosLock.Lock()
	defer _crtcInfosLock.Unlock()
	needRedraw = false
	if i, ok := _crtcInfos[crtc]; ok {
		// current crtc info has been saved,
		// redraw background only when crtc information changed
		if i.x != x || i.y != y || i.width != width || i.height != height {
			// update crtc info and redraw background
			Logger.Debug("update crtc info, old:", _crtcInfos[crtc])
			i.x = x
			i.y = y
			i.width = width
			i.height = height
			needRedraw = true
			Logger.Debug("update crtc info, new:", _crtcInfos[crtc])
		}
	} else {
		// current crtc info is out of save
		_crtcInfos[crtc] = &crtcInfo{
			x:      x,
			y:      y,
			width:  width,
			height: height,
		}
		needRedraw = true
		Logger.Debug("add crtc info:", _crtcInfos[crtc])
	}
	return
}

func removeCrtcInfos(crtc randr.Crtc) {
	_crtcInfosLock.Lock()
	defer _crtcInfosLock.Unlock()
	Logger.Debug("remove crtc info:", _crtcInfos[crtc])
	delete(_crtcInfos, crtc)
}

func listenBgFileChanged() {
	_bgGSettings.Connect("changed", func(s *gio.Settings, key string) {
		switch key {
		case gkeyCurrentBackground:
			Logger.Debug("background value in gsettings changed:", key)
			go mapBgToRoot()
			go func() {
				loadBgFile()
				drawBackground()
			}()
		}
	})
}

var XX randr.Crtc

func listenDisplayChanged() {
	_bgWinInfo.win.Listen(xproto.EventMaskExposure)
	randr.SelectInput(XU.Conn(), XU.RootWin(), randr.NotifyMaskCrtcChange|randr.NotifyMaskScreenChange)
	for {
		event, err := XU.Conn().WaitForEvent()
		if err != nil {
			continue
		}
		switch e := event.(type) {
		case xproto.ExposeEvent:
			// TODO
			Logger.Debug("expose event", e)
			go mapBgToRoot()
			drawBackground()
		case randr.ScreenChangeNotifyEvent:
			Logger.Debugf("ScreenChangeNotifyEvent: %dx%d", e.Width, e.Height)

			// FIXME skip invalid event for window manager issue
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
