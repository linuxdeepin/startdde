#include "./dbus/dbus_introspect.h"
#include "dwebview.h"
#include "background.h"
#include "display_info.h"

gboolean update_primary_info(struct DisplayInfo* info)
{
    GError* error = NULL;
    GDBusProxy* proxy = g_dbus_proxy_new_for_bus_sync(G_BUS_TYPE_SESSION,
                                                      G_DBUS_PROXY_FLAGS_NONE,
                                                      NULL,
                                                      DISPLAY_NAME,
                                                      DISPLAY_PATH,
                                                      DISPLAY_INTERFACE,
                                                      NULL,
                                                      &error
                                                      );
    if (error == NULL) {
        GVariant* res = g_dbus_proxy_get_cached_property(proxy, "PrimaryRect");
        g_variant_get(res, "(nnqq)", &info->x, &info->y, &info->width, &info->height);
        g_variant_unref(res);
        GVariant* primary = g_dbus_proxy_get_cached_property(proxy, "Primary");
        info->name = g_variant_get_string(primary,NULL);
        g_variant_unref(primary);
        g_debug("[%s] Display DBus primaryInfo::name:%s::%dx%d(%d,%d)",__func__, info->name, info->width, info->height, info->x, info->y);
        g_object_unref(proxy);
        return TRUE;
    } else {
        g_warning("[%s] connection dbus failed and use gdk to get primaryInfo: %s", __func__, error->message);
        g_clear_error(&error);
        GdkScreen* screen = gdk_screen_get_default();
        if (screen == NULL) return FALSE;
        gint num_primary = gdk_screen_get_primary_monitor(screen);
        GdkRectangle dest;
        gdk_screen_get_monitor_geometry(screen, num_primary, &dest);
        info->name = gdk_screen_get_monitor_plug_name(screen,num_primary);
        info->x = dest.x;
        info->y = dest.y;
        info->width = dest.width;
        info->height = dest.height;
        g_debug("[%s] gdk primaryInfo::name:%s::%dx%d(%d,%d)",__func__, info->name, info->width, info->height, info->x, info->y);
        return FALSE;
    }
}

gboolean update_screen_info(struct DisplayInfo* info)
{
    info->x = 0;
    info->y = 0;
    info->width = gdk_screen_width();
    info->height = gdk_screen_height();
    g_debug("[%s]:screen:%dx%d(%d,%d)",__func__, info->width, info->height, info->x, info->y);
    return TRUE;
}

gint update_monitors_num()
{
    GdkScreen* screen = gdk_screen_get_default();
    if (screen == NULL) return 0;
    return gdk_screen_get_n_monitors(screen);
}

void listen_primary_changed_signal(GDBusSignalCallback handler, gpointer data, GDestroyNotify data_free_func)
{
    GError* err = NULL;
    static GDBusConnection* conn = NULL;
    if (conn == NULL ) {
        conn = g_bus_get_sync(G_BUS_TYPE_SESSION, NULL, &err);
    }
    if (err != NULL) {
        g_warning("[%s] get dbus failed: %s", __func__, err->message);
        g_clear_error(&err);
        return;
    }
    add_watch(conn, DISPLAY_PATH, DISPLAY_INTERFACE, PRIMARY_CHANGED_SIGNAL);
    g_dbus_connection_signal_subscribe(conn,
                                       DISPLAY_NAME,
                                       DISPLAY_INTERFACE,
                                       PRIMARY_CHANGED_SIGNAL,
                                       DISPLAY_PATH,
                                       NULL,
                                       G_DBUS_SIGNAL_FLAGS_NONE,
                                       handler,
                                       data,
                                       data_free_func
                                       );
}

//Warning:
//the monitors-changed signal came before primaryChanged signal of Display dbus
//so if you want to use monitors-changed signal,you must use gdk primary to get primary rect instead of the Display Dbus PrimaryInfo
void listen_monitors_changed_signal(GCallback handler, gpointer data)
{
    GdkScreen* screen = gdk_screen_get_default();
    g_signal_connect(screen, "monitors-changed", G_CALLBACK(handler), data);
}

void widget_move_by_rect(GtkWidget* widget,struct DisplayInfo info)
{
    GdkGeometry geo = {0};
    geo.min_height = 0;
    geo.min_width = 0;

    gboolean realized = gtk_widget_get_realized(widget);
    g_debug("[%s] realized:==%d==, info: %d*%d(%d, %d)", __func__,realized, info.width, info.height, info.x, info.y);
    if (!realized) {
        gtk_window_set_geometry_hints(GTK_WINDOW(widget), NULL, &geo, GDK_HINT_MIN_SIZE);
        gtk_widget_set_size_request(widget, info.width, info.height);
        gtk_window_move(GTK_WINDOW(widget), info.x, info.y);
    }else {
        GdkWindow* gdk = gtk_widget_get_window(widget);
        gdk_window_set_geometry_hints(gdk, &geo, GDK_HINT_MIN_SIZE);
        gdk_window_move_resize(gdk, info.x, info.y,info.width,info.height );
        gdk_window_flush(gdk);
    }
}

void draw_background_by_rect(GtkWidget* widget,struct DisplayInfo info,const gchar* xatom_name)
{
    g_debug("[%s], %s:%dx%d(%d,%d)",__func__, info.name,info.width, info.height, info.x, info.y);

    gtk_widget_set_size_request(widget, info.width, info.height);
    gtk_window_move(GTK_WINDOW(widget), info.x, info.y);

    setup_background(widget,NULL,xatom_name);
    gtk_widget_realize (widget);
    GdkWindow* gdkwindow = gtk_widget_get_window (widget);
    gdk_window_set_accept_focus(gdkwindow,FALSE);
    gdk_window_set_override_redirect (gdkwindow, TRUE);
    gtk_widget_show (widget);
}
