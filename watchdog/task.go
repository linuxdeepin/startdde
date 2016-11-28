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
	"sync"
	"time"
)

const (
	loopDuration       = time.Second * 10
	admissibleDuration = time.Second * 2
)

type taskInfo struct {
	Name  string
	Times int // continuous launch times

	enabled       bool
	failed        bool
	prevTimestamp int64 // previous launch timestamp

	isRunning func() bool
	launcher  func() error

	locker sync.Mutex
}
type taskInfos []*taskInfo

func newTaskInfo(name string,
	isRunning func() bool, launcher func() error) *taskInfo {
	if isRunning == nil || launcher == nil {
		return nil
	}

	var task = &taskInfo{
		Name:          name,
		Times:         0,
		enabled:       true,
		failed:        false,
		prevTimestamp: time.Now().Unix(),
		isRunning:     isRunning,
		launcher:      launcher,
	}

	return task
}

func (task *taskInfo) Launch() error {
	if !task.CanLaunch() {
		task.Times = 0
		return nil
	}

	duration := time.Now().Unix() - task.prevTimestamp
	if duration < int64(loopDuration+admissibleDuration) {
		task.Times += 1
	} else {
		task.Times = 0
	}

	if maxLaunchTimes > 0 && task.Times == maxLaunchTimes {
		task.locker.Lock()
		task.failed = true
		task.locker.Unlock()
		logger.Debugf("Launch '%s' failed: over max launch times",
			task.Name)
	}

	task.prevTimestamp = time.Now().Unix()
	return task.launcher()
}

func (task *taskInfo) CanLaunch() bool {
	task.locker.Lock()
	if !task.enabled || task.failed {
		task.locker.Unlock()
		return false
	}
	task.locker.Unlock()

	return (task.isRunning() == false)
}

func (task *taskInfo) Over() bool {
	task.locker.Lock()
	defer task.locker.Unlock()
	return task.failed
}

func (task *taskInfo) Enable(enabled bool) {
	task.locker.Lock()
	defer task.locker.Unlock()
	if task.enabled == enabled {
		return
	}

	if enabled {
		task.failed = false
		task.Times = 0
	}
	task.enabled = enabled
}
