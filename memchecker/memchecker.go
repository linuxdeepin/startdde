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
