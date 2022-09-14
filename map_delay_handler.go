// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"sync"
	"time"
)

type mapDelayHandler struct {
	task  map[string]bool
	mutex sync.Mutex
	once  *sync.Once
	delay time.Duration
	do    func(string)
}

func newMapDelayHandler(delay time.Duration, f func(string)) *mapDelayHandler {
	return &mapDelayHandler{
		task:  make(map[string]bool),
		once:  &sync.Once{},
		do:    f,
		delay: delay,
	}
}

func (dh *mapDelayHandler) AddTask(name string) {
	dh.mutex.Lock()
	if _, ok := dh.task[name]; ok {
		dh.mutex.Unlock()
		return
	}
	dh.task[name] = true
	dh.mutex.Unlock()

	dh.once.Do(func() {
		logger.Debug("first do")
		time.AfterFunc(dh.delay, func() {
			if dh.do == nil {
				return
			}
			dh.mutex.Lock()

			for key := range dh.task {
				dh.do(key)
			}
			//clear dh.task
			dh.task = make(map[string]bool)

			logger.Debug("new once")
			dh.once = &sync.Once{}
			dh.mutex.Unlock()
		})
	})
}
