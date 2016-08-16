/**
 * Copyright (C) 2016 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package main

import (
	"gir/gio-2.0"
)

func main() {
	// preread all the items, so launcher daemon and dock daemon
	// can do this much faster.
	gio.AppInfoGetAll()
}
