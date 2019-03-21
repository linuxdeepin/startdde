package xcursor

/*
#cgo pkg-config: x11 xcursor xfixes
#include <X11/Xlib.h>
#include <X11/Xcursor/Xcursor.h>
#include <X11/extensions/Xfixes.h>
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"unsafe"
)

func LoadAndApply(theme, name string, size int) error {
	dpy := C.XOpenDisplay(nil)
	if dpy == nil {
		return errors.New("failed to open x display")
	}
	defer C.XCloseDisplay(dpy)

	cursor, err := loadCursor(dpy, theme, name, size)
	if err != nil {
		return err
	}

	rootWin := C.XDefaultRootWindow(dpy)
	C.XDefineCursor(dpy, rootWin, cursor)
	cName := C.CString(name)
	C.XFixesChangeCursorByName(dpy, cursor, cName)
	C.free(unsafe.Pointer(cName))
	C.XFreeCursor(dpy, cursor)
	return nil
}

func loadCursor(dpy *C.Display, theme, name string, size int) (C.Cursor, error) {
	cTheme := C.CString(theme)
	cName := C.CString(name)

	images := C.XcursorLibraryLoadImages(cName, cTheme, C.int(size))

	C.free(unsafe.Pointer(cTheme))
	C.free(unsafe.Pointer(cName))

	if images == nil {
		return 0, errors.New("failed to load x cursor images")
	}

	cursor := C.XcursorImagesLoadCursor(dpy, images)
	C.XcursorImagesDestroy(images)
	return cursor, nil
}
