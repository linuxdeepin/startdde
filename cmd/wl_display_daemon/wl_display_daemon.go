// SPDX-FileCopyrightText: 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"

	wl_display "github.com/linuxdeepin/startdde/wl_display"
)

func main() {
	err := wl_display.Start()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	select {}
}
