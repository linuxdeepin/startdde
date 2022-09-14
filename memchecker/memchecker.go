// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package memchecker

import (
	"fmt"
)

var (
	_config *configInfo
)

func init() {
	_config, _ = loadConfig(getConfigPath())
	if _config == nil {
		_config = &configInfo{
			MinMemAvail: defaultMinMemAvail,
			MaxSwapUsed: defaultMaxSwapUsed,
		}
	}
	correctConfig()
}

func GetConfig() *configInfo {
	return _config
}

// IsSufficient check the memory whether reaches the qualified value
func IsSufficient() bool {
	if _config.MinMemAvail == 0 {
		return true
	}

	info, err := GetMemInfo()
	if err != nil {
		return true
	}

	used := info.SwapTotal - info.SwapFree - info.SwapCached
	fmt.Printf("Avail: %v(%v), used: %v(%v)\n", info.MemAvailable,
		_config.MinMemAvail, used, _config.MaxSwapUsed)
	if info.MemAvailable < _config.MinMemAvail {
		return false
	}

	if _config.MaxSwapUsed == 0 {
		return true
	}

	if info.MemAvailable > used {
		return true
	}

	return (used < _config.MaxSwapUsed)
}

func correctConfig() {
	info, err := GetMemInfo()
	if err != nil {
		fmt.Println("Failed to get memory info:", err)
		return
	}

	_config.MaxSwapUsed *= 1024
	_config.MinMemAvail *= 1024
	if _config.MaxSwapUsed > info.SwapTotal {
		fmt.Printf("The max swap used invalid(%v, %v), try set to 0.25 of total\n",
			_config.MaxSwapUsed, info.SwapTotal)
		_config.MaxSwapUsed = uint64(float64(info.SwapTotal) * 0.25)
	}

	if _config.MinMemAvail > info.MemTotal {
		fmt.Printf("The min mem avail invalid(%v, %v), try set to 0.15 of total\n",
			_config.MinMemAvail, info.MemTotal)
		_config.MinMemAvail = uint64(float64(info.MemTotal) * 0.15)
	}
}
