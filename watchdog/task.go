// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package watchdog

import (
	"errors"
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

	isRunning   func() (bool, error)
	launch      func() error
	launchDelay time.Duration

	locker sync.Mutex
}

func newTaskInfo(name string,
	isRunning func() (bool, error), launcher func() error) *taskInfo {
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
		launch:        launcher,
		launchDelay:   time.Millisecond,
	}

	return task
}

func (task *taskInfo) Reset() {
	task.locker.Lock()
	task.Times = 0
	task.failed = false
	task.locker.Unlock()
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
	logger.Debug("launch task", task.Name, task.Times)
	return task.launch()
}

var errNoNeedLaunch = errors.New("no need launch")

func (task *taskInfo) CanLaunch() bool {
	task.locker.Lock()
	if !task.enabled || task.failed {
		task.locker.Unlock()
		return false
	}
	task.locker.Unlock()

	isRun, err := task.isRunning()
	if err != nil {
		if err != errNoNeedLaunch {
			logger.Warning(err)
		}
		return false
	}
	return !isRun
}

func (task *taskInfo) getFailed() bool {
	task.locker.Lock()
	defer task.locker.Unlock()
	return task.failed
}

func (task *taskInfo) GetFailed() bool {
	return task.getFailed()
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
