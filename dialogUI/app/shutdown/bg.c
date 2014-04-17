#include "bg.h"
#include "background.h"
#include "X_misc.h"

static Atom _BG_ATOM = 0; 

Drawable get_blurred_background()
{
    gulong n_item;
    gpointer data = get_window_property(gdk_x11_get_default_xdisplay(), GDK_ROOT_WINDOW(), _BG_ATOM, &n_item);
    if (data == NULL)
	return 0;
    Drawable bg = X_FETCH_32(data, 0);
    XFree(data);
    return bg;
}

GdkFilterReturn update_bg(XEvent* xevent, GdkEvent* event, BackgroundInfo* info)
{
    (void)event;
    if (xevent->type == PropertyNotify) {
	if (((XPropertyEvent*)xevent)->atom == _BG_ATOM) {
	    background_info_set_background_by_drawable(info, get_blurred_background());
	}
    }
    return GDK_FILTER_CONTINUE;
}

void setup_background(GtkWidget* container, GtkWidget* webview)
{
    _BG_ATOM = gdk_x11_get_xatom_by_name("_DDE_BACKGROUND_PIXMAP");

    BackgroundInfo* info = create_background_info(container, webview);
    background_info_set_background_by_drawable(info, get_blurred_background());

    gdk_window_set_events(gdk_get_default_root_window(), GDK_PROPERTY_CHANGE_MASK);
    gdk_window_add_filter(gdk_get_default_root_window(), (GdkFilterFunc)update_bg, info);
}
