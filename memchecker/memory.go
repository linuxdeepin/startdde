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

package memchecker

import (
	"bufio"
	"os"
	"pkg.deepin.io/lib/strv"
	"strconv"
	"strings"
)

// MemoryInfo show the current memory stat, sum by kb
type MemoryInfo struct {
	MemTotal     uint64
	MemFree      uint64
	MemAvailable uint64
	Buffers      uint64
	Cached       uint64
	SwapTotal    uint64
	SwapFree     uint64
	SwapCached   uint64
	MinAvailable uint64
	MaxSwapUsed  uint64
}

// GetMemInfo get the current memory used stat
func GetMemInfo() (*MemoryInfo, error) {
	info, err := doGetMemInfo("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	info.MinAvailable = _config.MinMemAvail
	info.MaxSwapUsed = _config.MaxSwapUsed
	return info, nil
}

func doGetMemInfo(filename string) (*MemoryInfo, error) {
	fr, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer fr.Close()

	var scanner = bufio.NewScanner(fr)
	var info = new(MemoryInfo)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		list := parseMemLine(line)
		if len(list) != 3 {
			continue
		}

		v := stou(list[1])
		switch {
		case strings.Contains(line, "MemTotal"):
			info.MemTotal = v
		case strings.Contains(line, "MemFree"):
			info.MemFree = v
		case strings.Contains(line, "MemAvailable"):
			info.MemAvailable = v
		case strings.Contains(line, "Buffers"):
			info.Buffers = v
		case strings.Contains(line, "Cached"):
			info.Cached = v
		case strings.Contains(line, "SwapTotal"):
			info.SwapTotal = v
		case strings.Contains(line, "SwapFree"):
			info.SwapFree = v
		case strings.Contains(line, "SwapCached"):
			info.SwapCached = v
		}
	}

	return info, nil
}

func parseMemLine(line string) []string {
	list := strings.Split(line, " ")
	list = strv.Strv(list).FilterEmpty()
	return list
}

func stou(s string) uint64 {
	if s == "0" {
		return 0
	}

	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}
