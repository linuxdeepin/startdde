/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
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

package watchdog

import (
	"time"

	"pkg.deepin.io/gir/gio-2.0"
	"pkg.deepin.io/lib/gsettings"
	dutils "pkg.deepin.io/lib/utils"
)

const (
	schemaId = "com.deepin.dde.watchdog"
)

type Manager struct {
	setting *gio.Settings
	quit    chan struct{}

	dbusTasks  map[string]*taskInfo
	timedTasks []*taskInfo
}

func newManager() *Manager {
	var m = new(Manager)
	m.quit = make(chan struct{})
	m.setting, _ = dutils.CheckAndNewGSettings(schemaId)
	m.dbusTasks = make(map[string]*taskInfo)
	return m
}

func (m *Manager) AddTimedTask(task *taskInfo) {
	task.Enable(m.getTaskEnabled(task.Name))
	m.timedTasks = append(m.timedTasks, task)
}

func (m *Manager) AddDBusTask(dbusServiceName string, task *taskInfo) {
	task.Enable(m.getTaskEnabled(task.Name))
	m.dbusTasks[dbusServiceName] = task
}

func (m *Manager) getTaskEnabled(taskName string) bool {
	if taskName == ddeLockTaskName {
		// force must be enabled
		return true
	}

	if m.setting != nil {
		return m.setting.GetBoolean(taskName)
	}
	return false
}

func (m *Manager) GetTask(name string) *taskInfo {
	for _, task := range m.timedTasks {
		if name == task.Name {
			return task
		}
	}
	for _, task := range m.dbusTasks {
		if name == task.Name {
			return task
		}
	}
	return nil
}

func (m *Manager) hasAnyRunnableTimedTask() bool {
	for _, task := range m.timedTasks {
		if !task.getFailed() {
			return true
		}
	}
	return false
}

func (m *Manager) launchAllTimedTasks() {
	for _, task := range m.timedTasks {
		err := task.Launch()
		if err != nil {
			logger.Warningf("Launch '%s' failed: %v",
				task.Name, err)
		}
	}
}

func (m *Manager) StartLoop() {
	m.handleSettingsChanged()
	for {
		select {
		case <-m.quit:
			return
		case _, ok := <-time.After(loopDuration):
			if !ok {
				logger.Error("Invalid time event")
				return
			}

			if !m.hasAnyRunnableTimedTask() {
				logger.Debug("All program has launched failure")
				m.QuitLoop()
				return
			}

			m.launchAllTimedTasks()
		}
	}
}

func (m *Manager) QuitLoop() {
	if m.quit == nil {
		return
	}
	if m.setting != nil {
		m.setting.Unref()
		m.setting = nil
	}
	close(m.quit)
	m.quit = nil
}

func (m *Manager) handleSettingsChanged() {
	if m.setting == nil {
		return
	}

	gsettings.ConnectChanged(schemaId, "*", func(key string) {
		task := m.GetTask(key)
		if task == nil {
			return
		}
		task.Enable(m.setting.GetBoolean(key))
	})
}
