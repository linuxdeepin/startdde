/**
 * Copyright (C) 2014 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

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

			for key, _ := range dh.task {
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
