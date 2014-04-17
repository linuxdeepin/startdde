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
	_ "image/jpeg"
	_ "image/png"
	"os"

	"dbus/com/deepin/daemon/display"
	"dlib/gio-2.0"
	"dlib/graphic"
	"fmt"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/render"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xprop"
	"github.com/BurntSushi/xgbutil/xwindow"
	"strings"
	"sync"
	"time"
)

const (
	personalizationID     = "com.deepin.dde.personalization"
	gkeyCurrentBackground = "current-picture"
	ddeBgPixmapProp       = "_DDE_BACKGROUND_PIXMAP"
	ddeBgPixmapBlurProp   = "_DDE_BACKGROUND_PIXMAP_BLURRED"
	ddeBgWindowTitle      = "DDE Background"
	defaultBackgroundFile = "/usr/share/backgrounds/default_background.jpg"
	tmpRootBgFile         = "/tmp/dde_bg.png"
	tmpRootBgBlurFile     = "/tmp/dde_bg_blur.png"
)

var (
	XU, _                           = xgbutil.NewConn()
	Display, _                      = display.NewDisplay("com.deepin.daemon.Display", "/com/deepin/daemon/Display")
	_gsettings                      = gio.NewSettings(personalizationID)
	_picFormat24, _picFormat32      render.Pictformat
	_filterNearest, _filterBilinear xproto.Str
	_filterConvolution              xproto.Str
	_bgwin                          *xwindow.Window
	_bgimg                          *xgraphics.Image
	_rootBgImg, _rootBgBlurImg      *xgraphics.Image
	_srcpid, _                      = render.NewPictureId(XU.Conn())
	_dstpid, _                      = render.NewPictureId(XU.Conn())
	_crtcInfos                      = make(map[randr.Crtc]*crtcInfo)
	_srcpidLock                     = sync.Mutex{}
	_crtcInfosLock                  = sync.Mutex{}
)

type crtcInfo struct {
	x      int16
	y      int16
	width  uint16
	height uint16
}

func initBackground() {
	randr.Init(XU.Conn())
	randr.QueryVersion(XU.Conn(), 1, 4)
	render.Init(XU.Conn())
	render.QueryVersion(XU.Conn(), 0, 11)

	_bgwin = createBgWindow(ddeBgWindowTitle)
	queryRender(xproto.Drawable(_bgwin.Id))

	// bind picture id to background window
	err := render.CreatePictureChecked(XU.Conn(), _dstpid, xproto.Drawable(_bgwin.Id), _picFormat24, 0, nil).Check()
	if err != nil {
		Logger.Error("create render picture failed: ", err)
	}

	listenBackgroundChanged()
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
		case "convolution":
			_filterConvolution = f
		}
	}
	Logger.Debug("nearest filter:", _filterNearest)
	Logger.Debug("bilinear filter:", _filterBilinear)
	Logger.Debug("convolution filter:", _filterConvolution)
}

func createBgWindow(title string) *xwindow.Window {
	win, err := xwindow.Generate(XU)
	if err != nil {
		Logger.Error("could not generate new window id: ", err)
		return nil
	}
	w, h := getScreenResolution()
	win.Create(XU.RootWin(), 0, 0, int(w), int(h), 0)

	// set _NET_WM_NAME so it looks nice
	err = ewmh.WmNameSet(XU, win.Id, title)
	if err != nil {
		// not a fatal error
		Logger.Error("Could not set _NET_WM_NAME: ", err)
	}

	// set _NET_WM_WINDOW_TYPE_DESKTOP window type
	ewmh.WmWindowTypeSet(XU, win.Id, []string{"_NET_WM_WINDOW_TYPE_DESKTOP"})

	win.Map()
	return win
}

func getScreenResolution() (w, h uint16) {
	screen := xproto.Setup(XU.Conn()).DefaultScreen(XU.Conn())
	w, h = screen.WidthInPixels, screen.HeightInPixels
	if w*h == 0 {
		// get root window geometry
		rootRect := xwindow.RootGeometry(XU)
		w, h = uint16(rootRect.Width()), uint16(rootRect.Height())
	}
	if w*h == 0 {
		w, h = 1024, 768 // default value
		Logger.Warningf("get screen resolution failed, use default value: %dx%d", w, h)
	}
	return
}

func getPrimaryScreenResolution() (w, h uint16) {
	defer func() {
		if err := recover(); err != nil {
			Logger.Error(err)
		}
	}()
	var value []interface{}
	value = Display.PrimaryRect.Get()
	if len(value) != 4 {
		Logger.Error("get primary rect failed", value)
		return 1024, 788
	}
	w, ok := value[2].(uint16)
	if !ok {
		Logger.Error("get primary screen resolution failed", Display)
		return 1024, 788
	}
	h, ok = value[3].(uint16)
	if !ok {
		Logger.Error("get primary screen resolution failed", Display)
		return 1024, 788
	}
	if w*h <= 0 {
		return 1024, 788
	}
	return
}

func updateBackground(delay bool) {
	defer func() {
		if err := recover(); err != nil {
			Logger.Error("updateBackground failed: ", err)
		}
	}()

	// load background file and convert into XU image
	_bgimg = convertToXimage(getBackgroundFile(), _bgimg)

	// rebind picture id to background pixmap
	Logger.Debugf("_srcpid=%d, _dstpid=%d", _srcpid, _dstpid)
	_srcpidLock.Lock()
	render.FreePicture(XU.Conn(), _srcpid)
	err := render.CreatePictureChecked(XU.Conn(), _srcpid, xproto.Drawable(_bgimg.Pixmap), _picFormat24, 0, nil).Check()
	if err != nil {
		Logger.Error("create render picture failed:", err)
		return
	}
	// setup image filter
	err = render.SetPictureFilterChecked(XU.Conn(), _srcpid, uint16(_filterBilinear.NameLen), _filterBilinear.Name, nil).Check()
	// TODO test only, for convolution filter
	// err = render.SetPictureFilterChecked(XU.Conn(), _srcpid, uint16(_filterConvolution.NameLen), _filterConvolution.Name, []render.Fixed{2621440, 2621440}).Check()
	// err = render.SetPictureFilterChecked(XU.Conn(), _srcpid, 1602*uint16(_filterConvolution.NameLen), _filterConvolution.Name, []render.Fixed{2621440, 2621440}).Check()
	if err != nil {
		Logger.Error("set picture filter failed:", err)
	}
	_srcpidLock.Unlock()

	updateAllScreens(delay)

	go mapBackgroundToRoot()
}

// load image file and return image.Image object.
func loadImage(imgfile string) (img image.Image, err error) {
	file, err := os.Open(imgfile)
	if err != nil {
		return
	}
	defer file.Close()
	img, _, err = image.Decode(file)
	if err != nil {
		Logger.Error("load image failed:", err)
	}
	return
}

// convert to XU image
func convertToXimage(imgFile string, ximg *xgraphics.Image) *xgraphics.Image {
	img, err := loadImage(imgFile)
	if err != nil {
		return ximg
	}
	if ximg != nil {
		ximg.Destroy()
	}
	ximg = xgraphics.NewConvert(XU, img)
	ximg.CreatePixmap()
	ximg.XDraw()
	return ximg
}

func getBgImgWidth() uint16 {
	if _bgimg == nil {
		Logger.Error("_bgimg is nil")
		return 1024
	}
	return uint16(_bgimg.Bounds().Max.X)
}
func getBgImgHeight() uint16 {
	if _bgimg == nil {
		Logger.Error("_bgimg is nil")
		return 768
	}
	return uint16(_bgimg.Bounds().Max.Y)
}

// TODO [re-implemented through xrender]
// TODO FIXME [error occurred if background file is busy]
func mapBackgroundToRoot() {
	defer func() {
		if err := recover(); err != nil {
			Logger.Error(err)
		}
	}()

	// generate temporary background file, same size with primary screen
	w, h := getPrimaryScreenResolution()
	err := graphic.FillImage(getBackgroundFile(), tmpRootBgFile, int32(w), int32(h), graphic.FillProportionCenterScale, graphic.PNG)
	if err != nil {
		panic(err)
	}

	// generate temporary blurred background file
	err = graphic.BlurImage(tmpRootBgFile, tmpRootBgBlurFile, 10, 10, graphic.PNG)
	if err != nil {
		panic(err)
	}

	_rootBgImg = convertToXimage(tmpRootBgFile, _rootBgImg)
	xprop.ChangeProp32(XU, XU.RootWin(), ddeBgPixmapProp, "PIXMAP", uint(_rootBgImg.Pixmap))

	_rootBgBlurImg = convertToXimage(tmpRootBgBlurFile, _rootBgBlurImg)
	xprop.ChangeProp32(XU, XU.RootWin(), ddeBgPixmapBlurProp, "PIXMAP", uint(_rootBgBlurImg.Pixmap))
}

func updateAllScreens(delay bool) {
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
		crtcReply, err := randr.GetCrtcInfo(XU.Conn(), reply.Crtc, 0).Reply()
		if err != nil {
			Logger.Warningf("get crtc info failed: id %d, %v", reply.Crtc, err)
			continue
		}
		updateCrtcInfos(reply.Crtc, crtcReply.X, crtcReply.Y, crtcReply.Width, crtcReply.Height)
		updateScreen(reply.Crtc, delay, false)
	}
}

func resizeBgWindow(w, h int) {
	geom, _ := _bgwin.Geometry()
	if geom.Width() == w && geom.Height() == h {
		return
	}
	Logger.Debugf("background window before resizing, %dx%d", geom.Width(), geom.Height())
	_bgwin.MoveResize(0, 0, w, h)
	geom, _ = _bgwin.Geometry()
	Logger.Debugf("background window after resizing, %dx%d", geom.Width(), geom.Height())
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

func updateScreen(crtc randr.Crtc, delay bool, drawDirectly bool) {
	if drawDirectly {
		go drawBackgroundDirectly(_srcpid, crtc)
	} else {
		if !delay {
			go drawBackgroundByRender(_srcpid, _dstpid, crtc)
		} else {
			go func() {
				// sleep 1s to ensure window resize event effected
				time.Sleep(1 * time.Second)
				drawBackgroundByRender(_srcpid, _dstpid, crtc)

				// sleep 5s and redraw background
				time.Sleep(5 * time.Second)
				drawBackgroundByRender(_srcpid, _dstpid, crtc)
			}()
		}
	}
}

// TODO [remove] draw background directly instead of through xrender, for that maybe
// issue with desktop manager after login
func drawBackgroundDirectly(_srcpid render.Picture, crtc randr.Crtc) {
	defer func() {
		if err := recover(); err != nil {
			Logger.Error("drawBackgroundDirectly failed:", err)
		}
	}()

	var x, y int16
	var width, height uint16
	if i, ok := _crtcInfos[crtc]; ok {
		x = i.x
		y = i.y
		width = i.width
		height = i.height
	} else {
		panic(fmt.Errorf("target crtc info is out of save: id=%d", crtc))
	}

	Logger.Debugf("draw background directly: x=%d, y=%d, width=%d, height=%d", x, y, width, height)

	// create temporary ximage
	ximg := xgraphics.New(XU, image.Rect(0, 0, int(width), int(height)))
	ximg.CreatePixmap()
	ximg.XDraw()

	// bind ximage to picture
	ximgpid, _ := render.NewPictureId(XU.Conn())
	err := render.CreatePictureChecked(XU.Conn(), ximgpid, xproto.Drawable(ximg.Pixmap), _picFormat24, 0, nil).Check()
	if err != nil {
		panic(err)
	}

	// draw background to ximage through xrender
	drawBackgroundByRender(_srcpid, ximgpid, crtc)

	// draw ximage to background window
	ximg.XSurfaceSet(_bgwin.Id)
	ximg.XDraw()
	ximg.XPaint(_bgwin.Id)

	// free resource
	render.FreePicture(XU.Conn(), ximgpid)
	ximg.Destroy()
}

func drawBackgroundByRender(_srcpid, _dstpid render.Picture, crtc randr.Crtc) {
	_srcpidLock.Lock()
	_crtcInfosLock.Lock()
	defer _srcpidLock.Unlock()
	defer _crtcInfosLock.Unlock()

	var x, y int16
	var width, height uint16
	if i, ok := _crtcInfos[crtc]; ok {
		x = i.x
		y = i.y
		width = i.width
		height = i.height
	} else {
		Logger.Errorf("target crtc info is out of save: id=%d", crtc)
		return
	}

	doDrawBackgroundByRender(_srcpid, _dstpid, x, y, width, height)
}

func doDrawBackgroundByRender(srcpid, dstpid render.Picture, x, y int16, width, height uint16) {
	defer func() {
		if err := recover(); err != nil {
			Logger.Error("doDrawBackgroundByRender() failed:", err)
		}
	}()

	Logger.Debugf("draw background through xrender: x=%d, y=%d, width=%d, height=%d", x, y, width, height)

	// get clip rectangle
	rect, err := getClipRect(width, height, getBgImgWidth(), getBgImgHeight())
	if err != nil {
		panic(err)
	}
	Logger.Debug("drawBackgroundByRender, clip rect", rect)

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

func getScaleTransform(x, y float32) render.Transform {
	return render.Transform{
		float32ToFixed(1 / x), 0, 0,
		0, float32ToFixed(1 / y), 0,
		0, 0, float32ToFixed(1),
	}
}

// convert float32 value to matrix fixed value
func float32ToFixed(f float32) render.Fixed {
	return render.Fixed(f * 65536)
}

// convert matrix fixed value to float32
func fixedToFloat32(f render.Fixed) float32 {
	return float32(f) / 65536
}

// get rectangle in image which with the same scale to reference
// width/heigh, and the rectangle will placed in center.
func getClipRect(refWidth, refHeight, imgWidth, imgHeight uint16) (rect xproto.Rectangle, err error) {
	x, y, w, h, err := graphic.GetProportionCenterScaleRect(int(refWidth), int(refHeight), int(imgWidth), int(imgHeight))
	rect.X = int16(x)
	rect.Y = int16(y)
	rect.Width = uint16(w)
	rect.Height = uint16(h)
	return
}

func getBackgroundFile() string {
	uri := _gsettings.GetString(gkeyCurrentBackground)
	Logger.Debug("background uri: ", uri)
	path, ok := uriToPath(uri)
	if !ok || !isFileExists(path) {
		Logger.Warning("background file is not exists")
		Logger.Warning("use default background: ", defaultBackgroundFile)
		return defaultBackgroundFile
	}
	return path
}

func uriToPath(uri string) (string, bool) {
	tmp := strings.TrimLeft(uri, " ")
	if strings.HasPrefix(tmp, "file://") {
		return tmp[7:], true
	}
	return "", false
}

func isFileExists(file string) bool {
	if _, err := os.Stat(file); err == nil {
		return true
	}
	return false
}

func listenBackgroundChanged() {
	_gsettings.Connect("changed", func(s *gio.Settings, key string) {
		switch key {
		case gkeyCurrentBackground:
			Logger.Debug("background value in gsettings changed: ", key)
			go updateBackground(false)
		}
	})
}

func listenDisplayChanged() {
	randr.SelectInput(XU.Conn(), XU.RootWin(), randr.NotifyMaskCrtcChange|randr.NotifyMaskScreenChange)
	for {
		event, err := XU.Conn().WaitForEvent()
		if err != nil {
			continue
		}
		switch eventType := event.(type) {
		case randr.NotifyEvent:
			switch eventType.SubCode {
			case randr.NotifyCrtcChange:
				info := eventType.U.Cc
				if info.Mode != 0 {
					Logger.Debugf("NotifyCrtcChange: update, id=%d, (%d,%d,%d,%d)",
						info.Crtc, info.X, info.Y, info.Width, info.Height)
					needRedraw := updateCrtcInfos(info.Crtc, info.X, info.Y, info.Width, info.Height)
					if needRedraw {
						updateScreen(info.Crtc, true, false)
					}
					// TODO
					// updateCrtcInfos(info.Crtc, info.X, info.Y, info.Width, info.Height)
					// updateScreen(info.Crtc, true, false)
				} else {
					Logger.Debugf("NotifyCrtcChange: remove, id=%d, (%d,%d,%d,%d)",
						info.Crtc, info.X, info.Y, info.Width, info.Height)
					removeCrtcInfos(info.Crtc)
				}
				// updateAllScreens(true)
			}
		case randr.ScreenChangeNotifyEvent:
			Logger.Debugf("ScreenChangeNotifyEvent: %dx%d", eventType.Width, eventType.Height)

			// FIXME skip invalid event for window manager issue
			if eventType.Width < 480 && eventType.Height < 640 {
				continue
			}

			resizeBgWindow(int(eventType.Width), int(eventType.Height))
		}
	}
}
