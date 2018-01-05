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
	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/log"
	"time"
)

const (
	dbusDest = "com.deepin.MemChecker"
	dbusPath = "/com/deepin/MemChecker"
	dbusIFC  = dbusDest
)

// MemChecker memory checker
type MemChecker struct {
	logger *log.Logger
	config *configInfo

	// indicate whether the memory is insufficient
	Insufficient bool
}

var _checker *MemChecker

// GetMemChecker return a mem checker object
func GetMemChecker(logger *log.Logger) *MemChecker {
	if _checker != nil {
		return _checker
	}
	_checker = newMemChecker(logger)
	return _checker
}

// Start launch and install memchecker
func Start(logger *log.Logger) error {
	checker := GetMemChecker(logger)
	err := dbus.InstallOnSession(checker)
	if err != nil {
		return err
	}
	dbus.DealWithUnhandledMessage()
	go checker.listen()
	return nil
}

func newMemChecker(logger *log.Logger) *MemChecker {
	var m = new(MemChecker)
	m.logger = logger
	m.config, _ = loadConfig(getConfigPath())
	if m.config == nil {
		m.config = &configInfo{
			MinMemAvail: defaultMinMemAvail,
			MaxSwapUsed: defaultMaxSwapUsed,
		}
	}
	m.correctConfig()

	return m
}

// GetMemoryStat return the current memory used stat
func (*MemChecker) GetMemoryStat() (*MemoryInfo, error) {
	return GetMemInfo()
}

// IsMemInsufficient check the memory whether reaches the qualified value
func (m *MemChecker) IsMemInsufficient() bool {
	info, err := GetMemInfo()
	if err != nil {
		m.logger.Warning("Failed to get memory info:", err)
		return false
	}

	used := info.SwapTotal - info.SwapFree
	m.logger.Debugf("Avail: %v, used: %v", info.MemAvailable, used)
	m.logger.Debug("Config:", m.config.MinMemAvail, m.config.MaxSwapUsed)
	if info.MemAvailable < m.config.MinMemAvail {
		return true
	}

	if m.config.MaxSwapUsed == 0 {
		return false
	}

	return (used > m.config.MaxSwapUsed)
}

func (*MemChecker) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		Dest:       dbusDest,
		ObjectPath: dbusPath,
		Interface:  dbusIFC,
	}
}

func (m *MemChecker) correctConfig() {
	info, err := GetMemInfo()
	if err != nil {
		m.logger.Warning("Failed to get memory info:", err)
		return
	}

	m.config.MaxSwapUsed *= 1024
	m.config.MinMemAvail *= 1024
	if m.config.MaxSwapUsed > info.SwapTotal {
		m.logger.Infof("The max swap used invalid(%v, %v), try set to 25% of total",
			m.config.MaxSwapUsed, info.SwapTotal)
		m.config.MaxSwapUsed = uint64(float64(info.SwapTotal) * 0.25)
	}

	if m.config.MinMemAvail > info.MemTotal {
		m.logger.Infof("The min mem avail invalid(%v, %v), try set to 15% of total",
			m.config.MinMemAvail, info.MemTotal)
		m.config.MinMemAvail = uint64(float64(info.MemTotal) * 0.15)
	}
}

func (m *MemChecker) listen() {
	for {
		time.Sleep(time.Second * 3)
		m.setPropInsufficient(m.IsMemInsufficient())
	}
}

func (m *MemChecker) setPropInsufficient(v bool) {
	if m.Insufficient == v {
		return
	}
	m.Insufficient = v
	dbus.NotifyChange(m, "Insufficient")
}
