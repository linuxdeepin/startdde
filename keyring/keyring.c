/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
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

#include <glib.h>
#include <gnome-keyring.h>

#include "keyring.h"

#define KEYRING_LOGIN "login"

static gboolean is_default_keyring(char *name);
static gboolean is_keyring_exist(char *name);

int
check_login()
{
    if (is_default_keyring(KEYRING_LOGIN)) {
        return 0;
    }

    GnomeKeyringResult r;
    if (!is_keyring_exist(KEYRING_LOGIN)) {
        // create login keyring
        r = gnome_keyring_create_sync(KEYRING_LOGIN, "");
        if (r != GNOME_KEYRING_RESULT_OK) {
            g_warning("Failed to create keyring: %d", r);
            return -1;
        }
    }

    // set login as default keyring
    r = gnome_keyring_set_default_keyring_sync(KEYRING_LOGIN);
    if (r != GNOME_KEYRING_RESULT_OK) {
        g_warning("Failed to set default keyring: %d", r);
        return -1;
    }
    return 0;
}

static gboolean
is_default_keyring(char *name)
{
    char *cur = NULL;
    GnomeKeyringResult r = gnome_keyring_get_default_keyring_sync(&cur);
    if (r != GNOME_KEYRING_RESULT_OK) {
        g_warning("Failed to get default keyring: %d", r);
        return TRUE;
    }

    if (!cur) {
        return FALSE;
    }

    g_debug("Default keyring: %s\n", cur);
    gboolean ret = g_str_equal(name, cur);
    g_free(cur);
    return ret;
}

static gboolean
is_keyring_exist(char *name)
{
    GList *list = NULL;
    GnomeKeyringResult r = gnome_keyring_list_keyring_names_sync(&list);
    if (r != GNOME_KEYRING_RESULT_OK) {
        g_warning("Failed to list keyring names: %d", r);
        return TRUE;
    }

    if (!list) {
        return FALSE;
    }

    GList *elem = NULL;
    gboolean ret = FALSE;
    for (elem = list; elem; elem = g_list_next(elem)){
        if (g_str_equal(name, (gchar*)(elem->data))) {
            ret = TRUE;
            break;
        }
    }
    g_list_free_full(list, g_free);
    return ret;
}
