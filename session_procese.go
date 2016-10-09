/**
 * Copyright (C) 2014 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package main

import "fmt"
import "io"
import "crypto/rand"
import "os"
import "os/exec"
import "time"

var launchTimeout = 30 * time.Second

func genUuid() string {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(rand.Reader, uuid)
	if n != len(uuid) || err != nil {
		panic("This can failed?")
	}
	// variant bits; see section 4.1.1
	uuid[8] = uuid[8]&^0xc0 | 0x80
	// version 4 (pseudo-random); see section 4.1.3
	uuid[6] = uuid[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}

func (m *SessionManager) launch(bin string, wait bool, args ...string) bool {
	id := genUuid()
	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()

	if !wait {
		go cmd.Run()
		return true
	}

	cmd.Env = append(cmd.Env, fmt.Sprintf("DDE_SESSION_PROCESS_COOKIE_ID=%s", id))
	m.cookieLocker.Lock()
	m.cookies[id] = make(chan time.Time, 1)
	m.cookieLocker.Unlock()
	startStamp := time.Now()

	err := cmd.Start()
	if err != nil {
		logger.Warningf("Start command '%s' failed: %v", bin, err)
		return false
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			logger.Warningf("Wait command '%s' failed: %v", bin, err)
		}
	}()

	select {
	case endStamp := <-m.cookies[id]:
		m.cookieLocker.Lock()
		delete(m.cookies, id)
		m.cookieLocker.Unlock()
		logger.Info(bin, "StartDuration:", endStamp.Sub(startStamp))
		return true
	case endStamp := <-time.After(launchTimeout):
		logger.Info(bin, "timeout:", endStamp.Sub(startStamp))
		return false
	}
}
func (m *SessionManager) Register(id string) bool {
	if cookie, ok := m.cookies[id]; ok {
		cookie <- time.Now()
		return true
	}
	return false
}
