/* -*- Mode: C; tab-width: 8; indent-tabs-mode: nil; c-basic-offset: 8 -*-
 *
 * Copyright (C) 2004-2006 William Jon McCann <mccann@jhu.edu>
 * Copyright (C) 2013      Linux Deepin Inc.
 *
 * This program is free software; you can redistribute it and/or
 * modify it under the terms of the GNU General Public License as
 * published by the Free Software Foundation; either version 2 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, write to the Free Software
 * Foundation, Inc., 59 Temple Place - Suite 330, Boston, MA
 * 02111-1307, USA.
 *
 * Authors: William Jon McCann <mccann@jhu.edu>
 *
 */

#include <stdlib.h>
#include <unistd.h>
#include <string.h>
#include <gdk/gdk.h>
#include <gdk/gdkx.h>
#include <gtk/gtk.h>

#include "gs-grab.h"

static void     gs_grab_class_init (GSGrabClass *klass);
static void     gs_grab_init       (GSGrab      *grab);
static void     gs_grab_finalize   (GObject        *object);

#define GS_GRAB_GET_PRIVATE(o) (G_TYPE_INSTANCE_GET_PRIVATE ((o), GS_TYPE_GRAB, GSGrabPrivate))

G_DEFINE_TYPE (GSGrab, gs_grab, G_TYPE_OBJECT)

static gpointer grab_object = NULL;

struct GSGrabPrivate
{
        guint      mouse_hide_cursor : 1;
        GdkWindow *mouse_grab_window;
        GdkWindow *keyboard_grab_window;
        GdkScreen *mouse_grab_screen;
        GdkScreen *keyboard_grab_screen;

        GtkWidget *invisible;
};

static int
gs_grab_get_keyboard (GSGrab    *grab,
                      GdkWindow *window,
                      GdkScreen *screen)
{
        g_return_val_if_fail (window != NULL, FALSE);
        g_return_val_if_fail (screen != NULL, FALSE);

        // FIXME: is this value ok?
        GdkGrabStatus status = GDK_GRAB_SUCCESS;
//gdk_keyboard_grab.
        GdkDisplay *display;
        GdkDeviceManager *device_manager;
        GdkDevice *device;
        GList *devices, *dev;

        display = gdk_window_get_display (window);
        device_manager = gdk_display_get_device_manager (display);
        devices = gdk_device_manager_list_devices (device_manager, GDK_DEVICE_TYPE_MASTER);
        for (dev = devices; dev; dev = dev->next)
        {
            device = dev->data;
            if (gdk_device_get_source (device) != GDK_SOURCE_KEYBOARD)
                continue;

            status = gdk_device_grab (device,
                                      window,
                                      GDK_OWNERSHIP_NONE,
                                      FALSE,
                                      GDK_KEY_PRESS_MASK | GDK_KEY_RELEASE_MASK,
                                      NULL,
                                      GDK_CURRENT_TIME
                                      );
        }
        g_list_free (devices);

        if (status == GDK_GRAB_SUCCESS) {
                if (grab->priv->keyboard_grab_window != NULL) {
                        g_object_remove_weak_pointer (G_OBJECT (grab->priv->keyboard_grab_window),
                                                      (gpointer *) &grab->priv->keyboard_grab_window);
                }
                grab->priv->keyboard_grab_window = window;

                g_object_add_weak_pointer (G_OBJECT (grab->priv->keyboard_grab_window),
                                           (gpointer *) &grab->priv->keyboard_grab_window);

                grab->priv->keyboard_grab_screen = screen;
        }

        return status;
}

static int
gs_grab_get_mouse (GSGrab    *grab,
                   GdkWindow *window,
                   GdkScreen *screen,
                   gboolean   hide_cursor)
{
        g_return_val_if_fail (window != NULL, FALSE);
        g_return_val_if_fail (screen != NULL, FALSE);

        // FIXME: is this value ok?
        GdkGrabStatus status = GDK_GRAB_SUCCESS;
        GdkCursor    *cursor;

        cursor = gdk_cursor_new (GDK_BLANK_CURSOR);

        g_debug ("Grabbing mouse widget=%X", (guint32) GDK_WINDOW_XID (window));
//gdk_pointer_grab
        GdkDisplay *display;
        GdkDeviceManager *device_manager;
        GdkDevice *device;
        GList *devices, *dev;

        display = gdk_window_get_display (window);
        device_manager = gdk_display_get_device_manager (display);
        devices = gdk_device_manager_list_devices (device_manager, GDK_DEVICE_TYPE_MASTER);

        for (dev = devices; dev; dev = dev->next)
        {
            device = dev->data;
            if (gdk_device_get_source (device) != GDK_SOURCE_MOUSE)
                continue;

            status = gdk_device_grab (device,
                                      window,
                                      GDK_OWNERSHIP_NONE,
                                      TRUE,
                                      0,
                                      (hide_cursor? cursor: NULL),
                                      GDK_CURRENT_TIME
                                      );
        }
        g_list_free (devices);
//

        if (status == GDK_GRAB_SUCCESS) {
                if (grab->priv->mouse_grab_window != NULL) {
                        g_object_remove_weak_pointer (G_OBJECT (grab->priv->mouse_grab_window),
                                                      (gpointer *) &grab->priv->mouse_grab_window);
                }
                grab->priv->mouse_grab_window = window;

                g_object_add_weak_pointer (G_OBJECT (grab->priv->mouse_grab_window),
                                           (gpointer *) &grab->priv->mouse_grab_window);

                grab->priv->mouse_grab_screen = screen;
                grab->priv->mouse_hide_cursor = hide_cursor;
        }

        g_object_unref (cursor);

        return status;
}

static gboolean
gs_grab_release_keyboard (GSGrab *grab)
{
        g_debug ("Ungrabbing keyboard");
//gdk_keyboard_ungrab
        GdkDisplay* display;
        GdkDeviceManager *device_manager;
        GList *devices, *dev;
        GdkDevice *device;

        display = gdk_display_get_default ();
        device_manager = gdk_display_get_device_manager (display);
        devices = gdk_device_manager_list_devices (device_manager, GDK_DEVICE_TYPE_MASTER);

        for (dev = devices; dev; dev = dev->next)
        {
            device = dev->data;
            if (gdk_device_get_source (device) != GDK_SOURCE_KEYBOARD)
                continue;

            gdk_device_ungrab (device, GDK_CURRENT_TIME);
        }
        g_list_free (devices);
//
        if (grab->priv->keyboard_grab_window != NULL) {
                g_object_remove_weak_pointer (G_OBJECT (grab->priv->keyboard_grab_window),
                                              (gpointer *) &grab->priv->keyboard_grab_window);
        }
        grab->priv->keyboard_grab_window = NULL;
        grab->priv->keyboard_grab_screen = NULL;

        return TRUE;
}

void
gs_grab_mouse_reset (GSGrab *grab)
{
        if (grab->priv->mouse_grab_window != NULL) {
                g_object_remove_weak_pointer (G_OBJECT (grab->priv->mouse_grab_window),
                                              (gpointer *) &grab->priv->mouse_grab_window);
        }

        grab->priv->mouse_grab_window = NULL;
        grab->priv->mouse_grab_screen = NULL;
}

gboolean
gs_grab_release_mouse (GSGrab *grab)
{
        g_debug ("Ungrabbing pointer");
//gdk_pointer_ungrab
        GdkDisplay* display;
        GdkDeviceManager *device_manager;
        GList *devices, *dev;
        GdkDevice *device;

        display = gdk_display_get_default ();
        device_manager = gdk_display_get_device_manager (display);
        devices = gdk_device_manager_list_devices (device_manager, GDK_DEVICE_TYPE_MASTER);

        for (dev = devices; dev; dev = dev->next)
        {
            device = dev->data;
            if (gdk_device_get_source (device) != GDK_SOURCE_MOUSE)
                continue;

            gdk_device_ungrab (device, GDK_CURRENT_TIME);
        }
        g_list_free (devices);
//
        gs_grab_mouse_reset (grab);

        return TRUE;
}


static void
gs_grab_nuke_focus (void)
{
        Window focus = 0;
        int    rev = 0;

        g_debug ("Nuking focus");

        gdk_error_trap_push ();

        XGetInputFocus (GDK_DISPLAY_XDISPLAY (gdk_display_get_default ()), &focus, &rev);

        XSetInputFocus (GDK_DISPLAY_XDISPLAY (gdk_display_get_default ()), None, RevertToNone, CurrentTime);

        gdk_error_trap_pop_ignored ();
}

void
gs_grab_release (GSGrab *grab)
{
        g_debug ("Releasing all grabs");

        gs_grab_release_mouse (grab);
        gs_grab_release_keyboard (grab);

        gdk_display_sync (gdk_display_get_default ());
        gdk_flush ();
}

gboolean
gs_grab_grab_window (GSGrab    *grab,
                     GdkWindow *window,
                     GdkScreen *screen,
                     gboolean   hide_cursor)
{
        gboolean mstatus = FALSE;
        gboolean kstatus = FALSE;
        int      i;
        int      retries = 4;
        gboolean focus_fuckus = FALSE;

 AGAIN:

        for (i = 0; i < retries; i++) {
                kstatus = gs_grab_get_keyboard (grab, window, screen);
                if (kstatus == GDK_GRAB_SUCCESS) {
                        break;
                }

                /* else, wait a second and try to grab again. */
                sleep (1);
        }

        if (kstatus != GDK_GRAB_SUCCESS) {
                if (!focus_fuckus) {
                        focus_fuckus = TRUE;
                        gs_grab_nuke_focus ();
                        goto AGAIN;
                }
        }

        for (i = 0; i < retries; i++) {
                mstatus = gs_grab_get_mouse (grab, window, screen, hide_cursor);
                if (mstatus == GDK_GRAB_SUCCESS) {
                        break;
                }

                /* else, wait a second and try to grab again. */
                sleep (1);
        }

        if (mstatus != GDK_GRAB_SUCCESS) {
                g_debug ("Couldn't grab pointer!");
        }

        /* When should we allow blanking to proceed?  The current theory
           is that both a keyboard grab and a mouse grab are mandatory

           - If we don't have a keyboard grab, then we won't be able to
           read a password to unlock, so the kbd grab is manditory.

           - If we don't have a mouse grab, then we might not see mouse
           clicks as a signal to unblank, on-screen widgets won't work ideally,
           and gs_grab_move_to_window() will spin forever when it gets called.
        */

        if (kstatus != GDK_GRAB_SUCCESS || mstatus != GDK_GRAB_SUCCESS) {
                /* Do not blank without a keyboard and mouse grabs. */

                /* Release keyboard or mouse which was grabbed. */
                if (kstatus == GDK_GRAB_SUCCESS) {
                        gs_grab_release_keyboard (grab);
                }
                if (mstatus == GDK_GRAB_SUCCESS) {
                        gs_grab_release_mouse (grab);
                }

                return FALSE;
        }

        /* Grab is good, go ahead and blank.  */
        return TRUE;
}

/* this is used to grab the keyboard and mouse to the root */
gboolean
gs_grab_grab_root (GSGrab  *grab,
                   gboolean hide_cursor)
{
        GdkDisplay *display;
        GdkWindow  *root;
        GdkScreen  *screen;
        GdkDeviceManager * device_manager;
        GdkDevice* pointer_device;
        gboolean    res;

        g_debug ("Grabbing the root window");
//gdk_pointer_get_position
        display = gdk_display_get_default ();
        device_manager = gdk_display_get_device_manager (display);
        pointer_device = gdk_device_manager_get_client_pointer (device_manager);
        gdk_device_get_position (pointer_device, &screen, NULL, NULL);
        root = gdk_screen_get_root_window (screen);

        res = gs_grab_grab_window (grab, root, screen, hide_cursor);

        return res;
}

/* This is similar to gs_grab_grab_window but doesn't fail */
void
gs_grab_move_to_window (GSGrab    *grab,
                        GdkWindow *window,
                        GdkScreen *screen,
                        gboolean   hide_cursor)
{
        g_return_if_fail (GS_IS_GRAB (grab));

        gboolean result = FALSE;

        GdkWindow *old_window;
        GdkScreen *old_screen;
        gboolean   old_hide_cursor;

        do {
            if (grab->priv->keyboard_grab_window == window) {
                result = TRUE;
            }
            else
            {
                gdk_x11_grab_server ();

                old_window = grab->priv->keyboard_grab_window;
                old_screen = grab->priv->keyboard_grab_screen;

                if (old_window) {
                    gs_grab_release_keyboard (grab);
                }

                result = gs_grab_get_keyboard (grab, window, screen);
                if (result != GDK_GRAB_SUCCESS) {
                    sleep (1);
                    result = gs_grab_get_keyboard (grab, window, screen);
                }
                if ((result != GDK_GRAB_SUCCESS) && old_window) {
                    g_debug ("Could not grab keyboard for new window.  Resuming previous grab.");
                    gs_grab_get_keyboard (grab, old_window, old_screen);
                }

                gdk_x11_ungrab_server ();
                gdk_flush ();

                result = (result == GDK_GRAB_SUCCESS);
            }
            gdk_flush ();
        } while (!result);

        do {
//gdk_pointer_is_grabbed
            GdkDisplay *display;
            GdkDeviceManager * device_manager;
            GdkDevice* pointer_device;
            display = gdk_display_get_default ();
            device_manager = gdk_display_get_device_manager (display);
            pointer_device = gdk_device_manager_get_client_pointer (device_manager);
            if (! gdk_display_device_is_grabbed (display, pointer_device)) {
                gs_grab_mouse_reset (grab);
            }

            if (grab->priv->mouse_grab_window == window) {
                result = TRUE;
            }
            else
            {
                gdk_x11_grab_server ();

                old_window = grab->priv->mouse_grab_window;
                old_screen = grab->priv->mouse_grab_screen;
                old_hide_cursor = grab->priv->mouse_hide_cursor;

                if (old_window) {
                    gs_grab_release_mouse (grab);
                }

                result = gs_grab_get_mouse (grab, window, screen, hide_cursor);
                if (result != GDK_GRAB_SUCCESS) {
                    sleep (1);
                    result = gs_grab_get_mouse (grab, window, screen, hide_cursor);
                }
                if ((result != GDK_GRAB_SUCCESS) && old_window) {
                    g_debug ("Could not grab mouse for new window.  Resuming previous grab.");
                    gs_grab_get_mouse (grab, old_window, old_screen, old_hide_cursor);
                }

                gdk_x11_ungrab_server ();
                gdk_flush ();

                result = (result == GDK_GRAB_SUCCESS);
            }
            gdk_flush ();
        } while (!result);
}

static void
gs_grab_class_init (GSGrabClass *klass)
{
        GObjectClass   *object_class = G_OBJECT_CLASS (klass);

        object_class->finalize = gs_grab_finalize;

        g_type_class_add_private (klass, sizeof (GSGrabPrivate));
}

static void
gs_grab_init (GSGrab *grab)
{
        grab->priv = GS_GRAB_GET_PRIVATE (grab);

        grab->priv->mouse_hide_cursor = FALSE;
        grab->priv->invisible = gtk_invisible_new ();
        gtk_widget_show (grab->priv->invisible);
}

static void
gs_grab_finalize (GObject *object)
{
        GSGrab *grab;

        g_return_if_fail (object != NULL);
        g_return_if_fail (GS_IS_GRAB (object));

        grab = GS_GRAB (object);

        g_return_if_fail (grab->priv != NULL);

        gtk_widget_destroy (grab->priv->invisible);

        G_OBJECT_CLASS (gs_grab_parent_class)->finalize (object);
}

GSGrab *
gs_grab_new (void)
{
        if (grab_object) {
                g_object_ref (grab_object);
        } else {
                grab_object = g_object_new (GS_TYPE_GRAB, NULL);
                g_object_add_weak_pointer (grab_object,
                                           (gpointer *) &grab_object);
        }

        return GS_GRAB (grab_object);
}

