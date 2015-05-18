package main

// #cgo pkg-config: gtk+-3.0
// #include <gtk/gtk.h>
// void gtkInit() {
//    gtk_init(NULL, NULL);
// }
import "C"
import (
	"flag"
	"pkg.linuxdeepin.com/lib/log"
	"pkg.linuxdeepin.com/lib/proxy"
)

var logger = log.NewLogger("com.deepin.SessionManager")

var debug = flag.Bool("d", false, "debug")
var windowManagerBin = flag.String("wm", "/usr/bin/deepin-wm", "the window manager used by dde")

func main() {
	C.gtkInit()
	flag.Parse()

	proxy.SetupProxy()

	startXSettings()

	startDisplay()

	startSession()

	C.gtk_main()
}
