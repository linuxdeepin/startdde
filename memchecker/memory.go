// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package memchecker

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/linuxdeepin/go-lib/strv"
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
		case list[0] == "MemTotal:":
			info.MemTotal = v
		case list[0] == "MemFree:":
			info.MemFree = v
		case list[0] == "MemAvailable:":
			info.MemAvailable = v
		case list[0] == "Buffers:":
			info.Buffers = v
		case list[0] == "Cached:":
			info.Cached = v
		case list[0] == "SwapTotal:":
			info.SwapTotal = v
		case list[0] == "SwapFree:":
			info.SwapFree = v
		case list[0] == "SwapCached:":
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
