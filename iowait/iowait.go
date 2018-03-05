/*
 * Copyright (C) 2018 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     kirigaya <kirigaya@mkacg.com>
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

package iowait

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"pkg.deepin.io/lib/log"
	"pkg.deepin.io/lib/strv"
)

// #cgo pkg-config: x11 xcursor xfixes gio-2.0
// #cgo LDFLAGS: -lm
// #include "xcursor_remap.h"
import "C"

const (
	ddeMaxIOWait = "DDE_MAX_IOWAIT"
)

var (
	_logger  *log.Logger
	cpuState CPUStat
	isWatch  = false
)

var _max = 65.0

func init() {
	s := os.Getenv(ddeMaxIOWait)
	if s == "" {
		return
	}

	v := stof(s)
	if v > 0 {
		_max = float64(v)
	}
}

// CPUStat store the cpu stat
type CPUStat struct {
	User   float64
	System float64
	Idle   float64
	IOWait float64
	Count  float64
}

// Start join the iowait module
func Start(logger *log.Logger) {
	_logger = logger
	for {
		time.Sleep(time.Second * 4)
		showIOWait()
	}
}

func showIOWait() {
	fr, err := os.Open("/proc/stat")
	if err != nil {
		_logger.Warning("Failed to open:", err)
		return
	}

	var scanner = bufio.NewScanner(fr)
	scanner.Scan()
	line := scanner.Text()
	fr.Close()
	list := strings.Split(line, " ")
	list = strv.Strv(list).FilterEmpty()
	if len(list) < 6 {
		_logger.Warning("INvalid format:", line, len(list))
		return
	}

	var TEMP CPUStat
	TEMP.User = stof(list[1])
	TEMP.System = stof(list[3])
	TEMP.Idle = stof(list[4])
	TEMP.IOWait = stof(list[5])
	TEMP.Count = (TEMP.User + TEMP.System + TEMP.Idle + TEMP.IOWait)

	if cpuState.Count == 0 {
		cpuState = TEMP
		return
	}

	count := TEMP.Count - cpuState.Count
	userStep := 100.0 * (TEMP.User - cpuState.User) / count
	sysStep := 100.0 * (TEMP.System - cpuState.System) / count
	iowaitStep := 100.0 * (TEMP.IOWait - cpuState.IOWait) / count

	_logger.Debug("current info: ", TEMP, userStep, sysStep, iowaitStep)
	xcLeftPtrToWatch(canShowWatch(userStep, sysStep, iowaitStep))
	cpuState = TEMP
}

func canShowWatch(user, sys, wait float64) bool {
	if user >= _max || sys >= _max || wait >= _max {
		return true
	}
	return false
}

func stof(v string) float64 {
	r, _ := strconv.ParseFloat(v, 64)
	return r
}

func xcLeftPtrToWatch(enabled bool) {
	if isWatch == enabled {
		return
	}

	var v C.int = 1
	if !enabled {
		v = 0
	}

	ret := C.xc_left_ptr_to_watch(v)
	if ret != 0 {
		fmt.Printf("Failed to map(%v) left_ptr/left_ptr_watch", enabled)
		return
	}
	isWatch = enabled
}
