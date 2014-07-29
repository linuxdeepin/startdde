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
	"github.com/BurntSushi/xgb/render"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xwindow"
	graphic "pkg.linuxdeepin.com/lib/gdkpixbuf"
	"pkg.linuxdeepin.com/lib/utils"
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
		logger.Errorf("get screen resolution failed, use default value: %dx%d", w, h)
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

func convertImageToXpixmap(imgFile string) (pix xproto.Pixmap, err error) {
	pix, err = graphic.ConvertImageToXpixmap(imgFile)
	if err != nil {
		logger.Error(err)
		return
	}
	// TODO
	// err = xcbPutXimage(xproto.Drawable(pix))
	// if err != nil {
	// 	return
	// }
	return
}

func xcbConvertImageToXpixmap(imgFile string) (pix xproto.Pixmap, err error) {
	ximg, err := xgraphics.NewFileName(XU, imgFile) // ~0.5s
	if err != nil {
		return
	}
	ximg.CreatePixmap()
	err = ximg.XDrawChecked()
	if err != nil {
		return
	}
	pix = ximg.Pixmap
	ximg.Pix = nil
	return
}

func xcbPutXimage(did xproto.Drawable) (err error) {
	ximg, err := xgraphics.NewDrawable(XU, did)
	if err != nil {
		logger.Error(err)
		return
	}
	defer func() {
		ximg.Pix = nil
	}()

	// TODO
	err = ximg.XDrawChecked()
	if err != nil {
		logger.Error(err)
		return
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
	x, y, w, h, err := graphic.GetPreferScaleClipRect(int(refWidth), int(refHeight), int(imgWidth), int(imgHeight))
	rect.X = int16(x)
	rect.Y = int16(y)
	rect.Width = uint16(w)
	rect.Height = uint16(h)
	return
}

func getBackgroundFile() string {
	uri := bgGSettings.GetString(gkeyCurrentBackground)
	logger.Debug("background uri:", uri)
	path := utils.DecodeURI(uri)
	if !utils.IsFileExist(path) {
		logger.Warning("background file is not exist:", path)
		logger.Warning("use default background:", defaultBackgroundFile)
		return defaultBackgroundFile
	}
	return path
}
