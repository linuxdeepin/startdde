/**
 * Copyright (c) 2014 Deepin, Inc.
 *               2014 Xu FaSheng
 *
 * Author:      Xu FaSheng <fasheng.xu@gmail.com>
 * Maintainer:  Xu FaSheng <fasheng.xu@gmail.com>
 *
 * This program is free software; you can redistribute it and/or
 * modify it under the terms of the GNU General Public License as
 * published by the Free Software Foundation; either version 3, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; see the file COPYING.  If not, write to
 * the Free Software Foundation, Inc., 51 Franklin Street, Fifth
 * Floor, Boston, MA 02110-1301, USA.
 **/

#include <stdio.h>
#include <glib.h>
#include <X11/Xlib.h>
#include <gdk-pixbuf/gdk-pixbuf.h>

Pixmap render_img_to_xpixmap(const char *img_path) {
        GError *err = NULL;
        GdkPixbuf *pixbuf = gdk_pixbuf_new_from_file(img_path, &err);
        if (err) {
                g_debug("new gdk pixbuf from file fialed: %s", err->message);
                g_error_free(err);
                return FALSE;
        }

        Pixmap xpixmap = 0;
        Display *display=XOpenDisplay(NULL);
        if (display == NULL) {
                g_debug("open display failed");
                return FALSE;
        }
        int screen_num=XDefaultScreen(display);
        gdk_pixbuf_xlib_init(display, screen_num);
        gdk_pixbuf_xlib_render_pixmap_and_mask(pixbuf, &xpixmap, NULL, 0);
        XCloseDisplay(display);

        g_object_unref(pixbuf);
        return xpixmap;
}
