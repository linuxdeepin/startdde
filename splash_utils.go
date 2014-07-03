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

// #cgo pkg-config: gdk-pixbuf-xlib-2.0 x11
// #cgo LDFLAGS: -lm
// #include <stdlib.h>
// #include "gdk_pixbuf_utils.h"
import "C"
import "unsafe"

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	"fmt"
	"github.com/BurntSushi/xgb/render"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xwindow"
	"net/url"
	"pkg.linuxdeepin.com/lib/graphic"
	"time"
)

func getBgImgWidth() uint16 {
	return bgImgInfo.width
}
func getBgImgHeight() uint16 {
	return bgImgInfo.height
}

func getScreenResolution() (w, h uint16) {
	screen := xproto.Setup(XU.Conn()).DefaultScreen(XU.Conn())
	w, h = screen.WidthInPixels, screen.HeightInPixels
	if w == 0 || h == 0 {
		// get root window geometry
		rootRect := xwindow.RootGeometry(XU)
		w, h = uint16(rootRect.Width()), uint16(rootRect.Height())
	}
	if w == 0 || h == 0 {
		w, h = 1024, 768 // default value
		logger.Error("get screen resolution failed, use default value: %dx%d", w, h)
	}
	return
}

func getPrimaryScreenResolution() (w, h uint16) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error(err)
		}
	}()

	// get Display.PrimaryRect, retry 20 times if read failed for that
	// display daemon maybe not ready
	var value []interface{}
	for i := 1; i < 50; i++ {
		var ok bool
		value, ok = getDisplayPrimaryRect()
		if !ok {
			logger.Warning("getPrimaryScreenResolution() retry", i)
			time.Sleep(200 * time.Millisecond)
			continue
		} else {
			break
		}
	}
	if len(value) != 4 {
		logger.Error("get primary rect failed", value)
		return 1024, 768
	}

	w, ok := value[2].(uint16)
	if !ok {
		logger.Error("get primary screen resolution failed", Display)
		return 1024, 768
	}
	h, ok = value[3].(uint16)
	if !ok {
		logger.Error("get primary screen resolution failed", Display)
		return 1024, 768
	}
	if w == 0 || h == 0 {
		logger.Error("get primary screen resolution failed", w, h, Display)
		return 1024, 768
	}
	return
}

func getDisplayPrimaryRect() (value []interface{}, ok bool) {
	done := make(chan int)
	go func() {
		value = Display.PrimaryRect.Get()
		done <- 0
	}()
	select {
	case <-time.After(200 * time.Millisecond):
		logger.Warning("getDisplayPrimaryRect() timeout")
	case <-done:
		if len(value) == 4 {
			logger.Info("getDisplayPrimaryRect() success:", value)
			ok = true
		}
	}
	return
}

// TODO
// convert image file to XU image
func convertToXimage(imgFile string) (ximg *xgraphics.Image, err error) {
	img, err := loadImage(imgFile) // ~0.4s
	if err != nil {
		return
	}
	ximg = xgraphics.NewConvert(XU, img) // ~0.2s
	ximg.CreatePixmap()
	ximg.XDraw()
	ximg.Pix = nil
	return
}

// TODO
// func convertToXpixmap(imgFile string) (pix xproto.Pixmap, err error) {
// 	ximg, err := convertToXimage(imgFile)
// 	pix = ximg.Pixmap
// 	return
// }

func convertToXpixmap(imgFile string) (pix xproto.Pixmap, err error) {
	cimgFile := C.CString(imgFile)
	defer C.free(unsafe.Pointer(cimgFile))
	pix = xproto.Pixmap(C.render_img_to_xpixmap(cimgFile))
	logger.Debug("render image to xpixmap:", pix)
	if pix <= 0 {
		err = fmt.Errorf("render image to xpixmap failed, %s", imgFile)
	}
	return
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
		logger.Error("load image failed:", err)
	}
	return
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
	uri := bgGSettings.GetString(gkeyCurrentBackground)
	logger.Debug("background uri:", uri)

	// decode url path, from
	// "file:///home/user/%E5%9B%BE%E7%89%87/Wallpapers/time%201.jpg"
	// to "/home/fsh/图片/Wallpapers/time 1.jpg"
	u, err := url.Parse(uri)
	if err != nil {
		logger.Error(err)
		return defaultBackgroundFile
	}
	path := u.Path

	if !isFileExists(path) {
		logger.Warning("background file is not exist:", path)
		logger.Warning("use default background:", defaultBackgroundFile)
		return defaultBackgroundFile
	}
	return path
}

func uriToPath(uri string) string {
	uri = strings.TrimLeft(uri, " ")
	uri = strings.TrimPrefix(uri, "file://")
	return uri
}

func isFileExists(file string) bool {
	if _, err := os.Stat(file); err == nil {
		return true
	}
	return false
}
