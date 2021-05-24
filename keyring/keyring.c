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

#include <stdbool.h>
#include <stdio.h>

#include <gio/gio.h>
#include <libsecret/secret.h>

#define KEYRING_LOGIN "login"
#define PASSWORD_SECRET_VALUE_CONTENT_TYPE "text/plain"
#define COLLECTION_INTERFACE "org.freedesktop.Secret.Collection"

static bool is_default_keyring_exists(SecretService *service);

int check_login() {
    int res = 0;

    GError *err = NULL;
    SecretService *service = NULL;
    SecretCollection *collection = NULL;
    SecretValue *password = NULL;
    GDBusConnection *bus = NULL;
    GVariant *ret = NULL;

    do {
        service = secret_service_get_sync(SECRET_SERVICE_OPEN_SESSION, NULL, &err);
        if (service == NULL) {
            printf("failed to get secret service: %s\n", err->message);
            res = 1;
            break;
        }

        if (is_default_keyring_exists(service)) {
            break;
        }

        password = secret_value_new("", 0, PASSWORD_SECRET_VALUE_CONTENT_TYPE);

        bus = g_bus_get_sync(G_BUS_TYPE_SESSION, NULL, &err);
        if (bus == NULL) {
            printf("failed to get session bus: %s\n", err->message);
            res = 1;
            break;
        }

        // create new collection without prompt
        GVariant *label = g_variant_new_dict_entry(
                g_variant_new_string(COLLECTION_INTERFACE ".Label"),
                g_variant_new_variant(g_variant_new_string(KEYRING_LOGIN)));
        GVariant *attributes = g_variant_new_array(G_VARIANT_TYPE("{sv}"), &label, 1);

        ret = g_dbus_connection_call_sync(
                bus,
                "org.gnome.keyring",
                "/org/freedesktop/secrets",
                "org.gnome.keyring.InternalUnsupportedGuiltRiddenInterface",
                "CreateWithMasterPassword",
                g_variant_new("(@a{sv}@(oayays))",
                              attributes,
                              secret_service_encode_dbus_secret(service, password)),
                NULL,
                G_DBUS_CALL_FLAGS_NONE,
                G_MAXINT,
                NULL,
                &err);
        if (err != NULL) {
            printf("failed to create keyring: %s\n", err->message);
            res = 1;
            break;
        }
    } while (false);

    if (err != NULL) g_error_free(err);
    if (service != NULL) g_object_unref(service);
    if (collection != NULL) g_object_unref(collection);
    if (password != NULL) secret_value_unref(password);
    if (bus != NULL) g_object_unref(bus);
    if (ret != NULL) g_variant_unref(ret);

    return res;
}

static bool is_default_keyring_exists(SecretService *service) {
    GError *err = NULL;
    SecretCollection *collection = secret_collection_for_alias_sync(service,
                                                                    SECRET_COLLECTION_DEFAULT,
                                                                    SECRET_COLLECTION_NONE,
                                                                    NULL,
                                                                    &err);
    if (err != NULL) {
        printf("failed to get default secret collection: %s\n", err->message);
        g_error_free(err);
        return false;
    }
    if (collection == NULL) {
        printf("default secret collection not exists\n");
        return false;
    }

    g_object_unref(collection);
    return true;
}
