package main

// #cgo pkg-config: x11 gtk+-3.0
// #include <X11/Xlib.h>
// #include <gtk/gtk.h>
// void gtkInit() {
//    XInitThreads();
//    gtk_init(NULL, NULL);
// }
import "C"
import (
	"flag"
	"pkg.deepin.io/dde/api/soundutils"
	"pkg.deepin.io/lib/log"
	"pkg.deepin.io/lib/proxy"
)

var logger = log.NewLogger("com.deepin.SessionManager")

var debug = flag.Bool("d", false, "debug")
var windowManagerBin = flag.String("wm", "/usr/bin/deepin-wm-switcher", "the window manager used by dde")

func main() {
	C.gtkInit()
	flag.Parse()

	soundutils.PlaySystemSound(soundutils.EventLogin, "", false)

	proxy.SetupProxy()

	startXSettings()

	startDisplay()

	startSession()

	C.gtk_main()
}
