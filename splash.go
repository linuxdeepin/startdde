// Example fullscreen shows how to make a window showing the Go Gopher go
// fullscreen and back using keybindings and EWMH.
package main

import (
	"image"
	_ "image/png"
	"log"
	"os"

	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xwindow"
)

func background() {
	X, err := xgbutil.NewConn()
	if err != nil {
		log.Fatal(err)
	}

	// Read an example gopher image into a regular png image.
	file, err := os.Open("/usr/share/backgrounds/default_background.jpg")
	defer file.Close()
	/*img, _, err := image.Decode(bytes.NewBuffer(gopher.GopherPng()))*/
	img, _, err := image.Decode(file)
	if err != nil {
		log.Fatal(err)
	}

	// Now convert it into an X image.
	ximg := xgraphics.NewConvert(X, img)

	// Now show it in a new window.
	// We set the window title and tell the program to quit gracefully when
	// the window is closed.
	// There is also a convenience method, XShow, that requires no parameters.
	win := showImage(ximg, "Deepin Background", true)
	ewmh.WmWindowTypeSet(win.X, win.Id, []string{"_NET_WM_WINDOW_TYPE_DESKTOP"})
}

// This is a slightly modified version of xgraphics.XShowExtra that does
// not set any resize constraints on the window (so that it can go
// fullscreen).
func showImage(im *xgraphics.Image, name string, quit bool) *xwindow.Window {
	if len(name) == 0 {
		name = "xgbutil Image Window"
	}
	w, h := im.Rect.Dx(), im.Rect.Dy()

	win, err := xwindow.Generate(im.X)
	if err != nil {
		xgbutil.Logger.Printf("Could not generate new window id: %s", err)
		return nil
	}

	// Create a very simple window with dimensions equal to the image.
	win.Create(im.X.RootWin(), 0, 0, w, h, 0)

	// Set _NET_WM_NAME so it looks nice.
	err = ewmh.WmNameSet(im.X, win.Id, name)
	if err != nil { // not a fatal error
		xgbutil.Logger.Printf("Could not set _NET_WM_NAME: %s", err)
	}

	// Paint our image before mapping.
	im.XSurfaceSet(win.Id)
	im.XDraw()
	im.XPaint(win.Id)

	// Now we can map, since we've set all our properties.
	// (The initial map is when the window manager starts managing.)
	win.Map()

	return win
}
