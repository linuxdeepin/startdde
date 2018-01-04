/*
 * Copyright (C) 2018 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     kirigaya <kirigaya@mkacg.com>
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

#include <stdio.h>
#include <string.h>

#include <gio/gio.h>

#include <X11/Xlib.h>
#include <X11/Xcursor/Xcursor.h>
#include <X11/extensions/Xfixes.h>

#include "xcursor_remap.h"

static int xc_remap(char *theme, char *src, char *dst, int size);

static XcursorImages*
xc_load_images(const char *theme, const char *name, int size)
{
    printf("Will load cursor file: %s - %s\n", theme, name);
    return XcursorLibraryLoadImages(name, theme, size);
}

static void
xc_change_cursor_by_name(Display *dpy, const char *theme,
                 const char *src, const char* dst, int size)
{
    if (size == -1) {
        size = XcursorGetDefaultSize(dpy);
    }

    // load cursor images
    XcursorImages *dst_images = xc_load_images(theme, dst, size);
    if (!dst_images) {
        XcursorImagesDestroy(dst_images);
        fprintf(stderr, "Failed to load cursor images: %s/%s\n", src, dst);
        return;
    }

    Cursor dst_cursor = XcursorImagesLoadCursor(dpy, dst_images);

    Window root = XDefaultRootWindow(dpy);
    XUndefineCursor(dpy, root);
    XDefineCursor(dpy, root, dst_cursor);
    printf("Will change cursor: %s --> %lu(%s)\n", src, dst_cursor, dst);
    XFixesChangeCursorByName(dpy, dst_cursor, src);

    XcursorImagesDestroy(dst_images);
    XFreeCursor(dpy, dst_cursor);
    return ;
}

static int
xc_remap(char *theme, char *src, char *dst, int size)
{
    Display *dpy = XOpenDisplay(NULL);
    if (!dpy) {
        fprintf(stderr, "Failed to open display\n");
        return -1;
    }

    xc_change_cursor_by_name(dpy, theme, src, dst, size);

    XCloseDisplay(dpy);
    return 0;
}

int
xc_left_ptr_to_watch(int enabled)
{
    char *theme = "deepin";
    int size = 24;

    GSettings *s = g_settings_new("com.deepin.xsettings");
    if (s) {
        theme = g_settings_get_string(s, "gtk-cursor-theme-name");
        size = g_settings_get_int(s, "gtk-cursor-theme-size");
        g_object_unref(s);
    }

    if (enabled) {
        return xc_remap("theme", "left_ptr", "watch", size);
    } else {
        return xc_remap("theme", "watch", "left_ptr", size);
    }
}
