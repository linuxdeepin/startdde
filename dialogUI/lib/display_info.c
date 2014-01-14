#include <dbus/dbus.h>
#include <dbus/dbus-glib.h>
#include <gio/gio.h>
#include <gdk/gdk.h>

#include "display_info.h"

gboolean update_display_info(struct DisplayInfo* info)
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
        g_debug("%dx%d(%d,%d)", info->width, info->height, info->x, info->y);
        g_object_unref(proxy);
        return TRUE;
    } else {
        g_warning("[%s] connection dbus failed: %s", __func__, error->message);
        g_clear_error(&error);
        info->x = 0;
        info->y = 0;
        info->width = gdk_screen_width();
        info->height = gdk_screen_height();
        return FALSE;
    }
}


void listen_primary_changed_signal(GSourceFunc handler)
{
    char* rules = g_strdup_printf("eavesdrop='true',"
                                  "type='signal',"
                                  "interface='%s',"
                                  "member='%s',"
                                  "path='%s'",
                                  DISPLAY_INTERFACE,
                                  PRIMARY_CHANGED_SIGNAL,
                                  DISPLAY_PATH);
    g_debug("rules: %s", rules);

    DBusError error;
    dbus_error_init(&error);

    DBusConnection* conn = dbus_bus_get(DBUS_BUS_SESSION, &error);

    if (dbus_error_is_set(&error)) {
        g_warning("[%s] Connection Error: %s", __func__, error.message);
        dbus_error_free(&error);
        return;
    }

    dbus_bus_add_match(conn, rules, &error);
    g_free(rules);

    if (dbus_error_is_set(&error)) {
        g_warning("[%s] add match failed: %s", __func__, error.message);
        dbus_error_free(&error);
        return;
    }

    dbus_connection_flush(conn);

    g_debug("[%s] listen update signal", __func__);
    g_timeout_add(100, (GSourceFunc)handler, conn);
}

