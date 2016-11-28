/**
 * Copyright (C) 2016 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package watchdog

import (
	"gir/gio-2.0"
	dutils "pkg.deepin.io/lib/utils"
	"time"
)

const (
	schemaId = "com.deepin.dde.watchdog"
)

type Manager struct {
	taskList *taskInfos
	setting  *gio.Settings
	quit     chan struct{}
}

func newManager() *Manager {
	var m = new(Manager)
	m.quit = make(chan struct{})
	m.taskList = new(taskInfos)
	m.setting, _ = dutils.CheckAndNewGSettings(schemaId)
	return m
}

func (m *Manager) AddTask(task *taskInfo) {
	if m.IsTaskExist(task.Name) {
		logger.Debugf("Task '%s' has exist", task.Name)
		return
	}

	if m.setting != nil {
		task.Enable(m.setting.GetBoolean(task.Name))
	}

	*m.taskList = append(*m.taskList, task)
}

func (m *Manager) IsTaskExist(name string) bool {
	return (m.GetTask(name) != nil)
}

func (m *Manager) GetTask(name string) *taskInfo {
	for _, task := range *m.taskList {
		if name == task.Name {
			return task
		}
	}
	return nil
}

func (m *Manager) HasRunning() bool {
	for _, task := range *m.taskList {
		if !task.Over() {
			return true
		}
	}
	return false
}

func (m *Manager) LaunchAll() {
	for _, task := range *m.taskList {
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
		case <-time.After(loopDuration):
			if !m.HasRunning() {
				logger.Debug("All program has launched failure")
				m.QuitLoop()
				return
			}

			m.LaunchAll()
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

	m.setting.Connect("changed", func(s *gio.Settings, key string) {
		switch key {
		case dockName, desktopName:
			task := m.GetTask(key)
			if task == nil {
				return
			}
			task.Enable(m.setting.GetBoolean(key))
		}
	})
	m.setting.GetBoolean(dockName)
}
