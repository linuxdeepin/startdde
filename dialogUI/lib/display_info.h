#ifndef _DISPLAY_INFO_H_
#define _DISPLAY_INFO_H_

#include <glib.h>

#define DISPLAY_NAME "com.deepin.daemon.Display"
#define DISPLAY_PATH "/com/deepin/daemon/Display"
#define DISPLAY_INTERFACE DISPLAY_NAME
#define PRIMARY_CHANGED_SIGNAL "PrimaryChanged"

struct DisplayInfo {
    gint16 x, y;
    guint16 width, height;
};

gboolean update_display_info(struct DisplayInfo* info);
void listen_primary_changed_signal(GSourceFunc handler);

#endif /* end of include guard: _DISPLAY_INFO_H_ */

