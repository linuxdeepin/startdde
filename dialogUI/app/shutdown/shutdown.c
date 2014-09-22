/**
 * Copyright (c) 2011 ~ 2014 Deepin, Inc.
 *               2011 ~ 2014 bluth
 *
 * Author:      bluth <yuanchenglu001@gmail.com>
 * Maintainer:  bluth <yuanchenglu001@gmail.com>
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
#include <gtk/gtk.h>
#include <cairo-xlib.h>
#include <gdk/gdkx.h>
#include <gdk-pixbuf/gdk-pixbuf.h>
#include <unistd.h>
#include <glib.h>
#include <stdlib.h>
#include <string.h>
#include <glib/gstdio.h>
#include <glib/gprintf.h>
#include <sys/types.h>
#include <signal.h>
#include <X11/XKBlib.h>

#include "jsextension.h"
#include "dwebview.h"
#include "background.h"
#include "display_info.h"

#include "X_misc.h"
#include "gs-grab.h"
#include "i18n.h"
#include "utils.h"


#define SHUTDOWN_ID_NAME "deepin.dde.shutdown"
#define CHOICE_HTML_PATH "file://"RESOURCE_DIR"/shutdown/index.html"

#define SHUTDOWN_MAJOR_VERSION 2
#define SHUTDOWN_MINOR_VERSION 0
#define SHUTDOWN_SUBMINOR_VERSION 0
#define SHUTDOWN_VERSION G_STRINGIFY(SHUTDOWN_MAJOR_VERSION)"."G_STRINGIFY(SHUTDOWN_MINOR_VERSION)"."G_STRINGIFY(SHUTDOWN_SUBMINOR_VERSION)
#define SHUTDOWN_CONF "shutdown/config.ini"
static GKeyFile* shutdown_config = NULL;

#ifdef NDEBUG
static GSGrab* grab = NULL;
#endif
PRIVATE GtkWidget* bg_window = NULL;
PRIVATE GtkWidget* container = NULL;
PRIVATE GtkWidget* webview = NULL;

#ifdef NDEBUG
PRIVATE
gint t_id;
#endif

PRIVATE struct DisplayInfo rect_primary;
PRIVATE struct DisplayInfo rect_screen;

void notify_workarea_size(struct DisplayInfo info)
{
    JSObjectRef size_info = json_create();
    json_append_number(size_info, "x", info.x);
    json_append_number(size_info, "y", info.y);
    json_append_number(size_info, "width", info.width);
    json_append_number(size_info, "height", info.height);
    js_post_message("workarea_size_changed", size_info);
}

JS_EXPORT_API
void shutdown_emit_webview_ok()
{
    update_primary_info(&rect_primary);
    notify_workarea_size(rect_primary);
}


JS_EXPORT_API
void shutdown_quit()
{
    g_key_file_free(shutdown_config);
    gtk_main_quit();
}

JS_EXPORT_API
void shutdown_restack()
{
    gdk_window_restack(gtk_widget_get_window(container), NULL, TRUE);
}


#ifdef NDEBUG
static void
focus_out_cb (GtkWidget* w G_GNUC_UNUSED, GdkEvent* e G_GNUC_UNUSED, gpointer user_data G_GNUC_UNUSED)
{
    g_debug("[%s]:====",__func__);
    gdk_window_focus (gtk_widget_get_window (container), 0);
}

gboolean gs_grab_move ()
{
    g_message("timeout grab==============");
    gs_grab_move_to_window (grab,
                            gtk_widget_get_window (container),
                            gtk_window_get_screen (GTK_WINDOW(container)),
                            FALSE);
    /*gtk_timeout_remove(t_id);*/
    return FALSE;
}

static void
show_cb ()
{
    t_id = g_timeout_add(500,(GSourceFunc)gs_grab_move,NULL);
}

#endif

PRIVATE
void check_version()
{
    if (shutdown_config == NULL)
        shutdown_config = load_app_config(SHUTDOWN_CONF);

    GError* err = NULL;
    gchar* version = g_key_file_get_string(shutdown_config, "main", "version", &err);
    if (err != NULL) {
        g_warning("[%s] read version failed from config file: %s", __func__, err->message);
        g_error_free(err);
        g_key_file_set_string(shutdown_config, "main", "version", SHUTDOWN_VERSION);
        save_app_config(shutdown_config, SHUTDOWN_CONF);
    }

    if (version != NULL)
        g_free(version);
}


JS_EXPORT_API
const char* shutdown_get_username()
{
    const gchar *username = g_get_user_name();
    return username;
}

PRIVATE
void spawn_command_sync (const char* command,gboolean sync)
{
    GError *error = NULL;
    const gchar *cmd = g_strdup_printf ("%s",command);
    if(sync){
        g_message ("g_spawn_command_line_sync:%s",cmd);
        g_spawn_command_line_sync (cmd, NULL, NULL, NULL, &error);
    }else{
        g_message ("g_spawn_command_line_async:%s",cmd);
        g_spawn_command_line_async (cmd, &error);
    }
    if (error != NULL) {
        g_warning ("%s failed:%s\n",cmd, error->message);
        g_error_free (error);
        error = NULL;
    }
}

JS_EXPORT_API
void shutdown_switch_to_greeter(){
    spawn_command_sync("/usr/bin/dde-switchtogreeter",FALSE);
}

JS_EXPORT_API
gboolean shutdown_is_debug(){
#ifdef NDEBUG
    return FALSE;
#endif
    return TRUE;
}

PRIVATE
void monitors_changed_cb()
{
    g_debug("[%s] signal========",__func__);
    update_primary_info(&rect_primary);
    update_screen_info(&rect_screen);

    widget_move_by_rect(container,rect_primary);
    draw_background_by_rect(bg_window,rect_screen,"_DDE_BACKGROUND_WINDOW");
}

int main (int argc, char **argv)
{
    if (argc == 2 && 0 == g_strcmp0(argv[1], "-d")){
        g_message("dde-shutdown -d");
        g_setenv("G_MESSAGES_DEBUG", "all", FALSE);
    }
    if (is_application_running(SHUTDOWN_ID_NAME)) {
        g_warning("another instance of application dde-shutdown is running...\n");
        return 0;
    }

    singleton(SHUTDOWN_ID_NAME);

    check_version();
    init_i18n ();

    gtk_init (&argc, &argv);
    g_log_set_default_handler((GLogFunc)log_to_file, "dde-shutdown");

    update_primary_info(&rect_primary);
    update_screen_info(&rect_screen);

    bg_window = gtk_window_new (GTK_WINDOW_TOPLEVEL);
    draw_background_by_rect(bg_window,rect_screen,"_DDE_BACKGROUND_WINDOW");

    container = create_web_container (FALSE, TRUE);
    widget_move_by_rect(container,rect_primary);
    listen_primary_changed_signal(monitors_changed_cb,NULL,NULL);

    webview = d_webview_new_with_uri (CHOICE_HTML_PATH);
    gtk_container_add (GTK_CONTAINER(container), GTK_WIDGET (webview));

    gtk_widget_set_events (container,GDK_ALL_EVENTS_MASK);
    gtk_widget_set_events (webview,GDK_ALL_EVENTS_MASK);

    g_signal_connect(webview, "draw", G_CALLBACK(erase_background), NULL);
#ifdef NDEBUG
    grab = gs_grab_new ();
    g_message("Shutdown Not DEBUG");
    g_signal_connect (container, "show", G_CALLBACK (show_cb), NULL);
    g_signal_connect (webview, "focus-out-event", G_CALLBACK( focus_out_cb), NULL);
#endif
    gtk_widget_realize (container);
    gtk_widget_realize (webview);

    GdkWindow* gdkwindow = gtk_widget_get_window (container);
    gdk_window_set_accept_focus(gdkwindow,TRUE);
    gdk_window_set_keep_above (gdkwindow, TRUE);
    gdk_window_set_override_redirect (gdkwindow, TRUE);//must

    gtk_widget_show_all (container);

    gtk_main ();

    return 0;
}
