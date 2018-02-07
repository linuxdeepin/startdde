/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
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

package display

import (
	"fmt"
)

func (m *MonitorInfo) Enable(enabled bool) error {
	m.locker.Lock()
	defer m.locker.Unlock()
	err := m.doEnable(enabled)
	if err != nil {
		logger.Warningf("Toggle '%s' to '%v' failed: %v", m.Name, enabled, err)
		return err
	}
	m.cfg.Enabled = enabled
	_dpy.detectHasChanged()
	return nil
}

func (m *MonitorInfo) SetMode(v uint32) error {
	m.locker.Lock()
	defer m.locker.Unlock()
	// TODO: Why needs to modify monitor properties? Try only modify cfg properties.
	err := m.doSetMode(v)
	if err != nil {
		logger.Warningf("set mode '%v' for '%s' failed: %v", v, m.Name, err)
		return err
	}
	m.cfg.Width, m.cfg.Height, m.cfg.RefreshRate = m.CurrentMode.Width, m.CurrentMode.Height, m.CurrentMode.Rate
	_dpy.detectHasChanged()
	return nil
}

func (m *MonitorInfo) SetModeBySize(w, h uint16) error {
	matches := m.Modes.QueryBySize(w, h)
	if len(matches) == 0 {
		logger.Warning("Invalid mode size:", w, h)
		return fmt.Errorf("The mode size %dx%d invalid", w, h)
	}

	return m.SetMode(matches[0].Id)
}

func (m *MonitorInfo) SetRefreshRate(rate float64) error {
	m.locker.Lock()
	defer m.locker.Unlock()
	err := m.doSetRefreshRate(rate)
	if err != nil {
		logger.Warning(err)
		return err
	}
	m.cfg.RefreshRate = rate
	_dpy.detectHasChanged()
	return nil
}

func (m *MonitorInfo) SetPosition(x, y int16) error {
	m.locker.Lock()
	defer m.locker.Unlock()
	m.doSetPosition(x, y)
	m.cfg.X, m.cfg.Y = x, y
	_dpy.detectHasChanged()
	return nil
}

func (m *MonitorInfo) SetRotation(v uint16) error {
	m.locker.Lock()
	defer m.locker.Unlock()
	err := m.doSetRotation(v)
	if err != nil {
		logger.Warningf("Set rotation '%v' for '%s' failed: %v", v, m.Name, err)
		return err
	}
	m.cfg.Rotation = v
	_dpy.detectHasChanged()
	return nil
}

func (m *MonitorInfo) SetReflect(v uint16) error {
	m.locker.Lock()
	defer m.locker.Unlock()
	err := m.doSetReflect(v)
	if err != nil {
		logger.Warningf("Set reflect '%v' for '%s' failed: %v", v, m.Name, err)
		return err
	}
	m.cfg.Reflect = v
	_dpy.detectHasChanged()
	return nil
}
