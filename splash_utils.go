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
	"strings"

	"dlib/graphic"
	"github.com/BurntSushi/xgb/render"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xwindow"
)

func getBgImgWidth() uint16 {
	if _bgImgInfo.img == nil {
		Logger.Error("_bgImgInfo.img is nil")
		return 1024
	}
	return uint16(_bgImgInfo.img.Bounds().Max.X)
}
func getBgImgHeight() uint16 {
	if _bgImgInfo.img == nil {
		Logger.Error("_bgImgInfo.img is nil")
		return 768
	}
	return uint16(_bgImgInfo.img.Bounds().Max.Y)
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

// convert image file to XU image
func convertToXimage(imgFile string, ximg *xgraphics.Image) *xgraphics.Image {
	img, err := loadImage(imgFile) // ~0.4s
	if err != nil {
		return ximg
	}
	if ximg != nil {
		ximg.Destroy()
	}
	ximg = xgraphics.NewConvert(XU, img) // ~0.2s
	ximg.CreatePixmap()
	ximg.XDraw()
	return ximg
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
	uri := _bgGSettings.GetString(gkeyCurrentBackground)
	Logger.Debug("background uri:", uri)
	path, ok := uriToPath(uri)
	if !ok || !isFileExists(path) {
		Logger.Warning("background file is not exist:", path)
		Logger.Warning("use default background:", defaultBackgroundFile)
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
