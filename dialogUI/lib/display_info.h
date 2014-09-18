#ifndef _DISPLAY_INFO_H_
#define _DISPLAY_INFO_H_

#include <glib.h>
#include <gio/gio.h>
#include <gtk/gtk.h>

#define DISPLAY_NAME "com.deepin.daemon.Display"
#define DISPLAY_PATH "/com/deepin/daemon/Display"
#define DISPLAY_INTERFACE DISPLAY_NAME
#define PRIMARY_CHANGED_SIGNAL "PrimaryChanged"

struct DisplayInfo {
    gint16 x, y;
    guint16 width, height;
    const gchar* name;
};

gboolean update_primary_info(struct DisplayInfo* info);
gboolean update_screen_info(struct DisplayInfo* info);

void listen_primary_changed_signal(GDBusSignalCallback handler, gpointer data, GDestroyNotify data_free_func);
void listen_monitors_changed_signal(GCallback handler, gpointer data);

gint update_monitors_num();

void widget_move_by_rect(GtkWidget* widget,struct DisplayInfo info);

void draw_background_by_rect(GtkWidget* widget,struct DisplayInfo info,const gchar* xatom_name);

#endif /* end of include guard: _DISPLAY_INFO_H_ */
