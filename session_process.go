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

package main

import (
	"fmt"

	"pkg.deepin.io/lib/dbus"
)
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

func (m *SessionManager) startSessionDaemonPart2() bool {
	sessionBus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return false
	}

	timeStart := time.Now()
	sessionDaemonObj := sessionBus.Object("com.deepin.daemon.Daemon", "/com/deepin/daemon/Daemon")
	err = sessionDaemonObj.Call("com.deepin.daemon.Daemon.StartPart2",
		dbus.FlagNoAutoStart).Err
	logger.Info("start dde-session-daemon part2 cost:", time.Since(timeStart))
	m.allowSessionDaemonRun = true

	if err != nil {
		logger.Warning(err)
		return false
	}
	return true
}

func (m *SessionManager) launchWait(bin string, args ...string) bool {
	id := genUuid()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "DDE_SESSION_PROCESS_COOKIE_ID="+id)

	ch := make(chan time.Time, 1)
	m.cookieLocker.Lock()
	m.cookies[id] = ch
	m.cookieLocker.Unlock()

	cmdStr := fmt.Sprintf("%s %v", bin, args)
	timeStart := time.Now()

	err := cmd.Start()
	if err != nil {
		logger.Warningf("Start command %s failed: %v", cmdStr, err)
		return false
	}
	logger.Debug("pid:", cmd.Process.Pid)
	go func() {
		err := cmd.Wait()
		if err != nil {
			logger.Warningf("Wait command %s failed: %v", cmdStr, err)
		}
		m.cookieLocker.Lock()
		timeCh := m.cookies[id]
		if timeCh != nil {
			delete(m.cookies, id)
			timeCh <- time.Now()
		}
		m.cookieLocker.Unlock()
	}()

	select {
	case timeEnd := <-ch:
		logger.Info(cmdStr, "startup duration:", timeEnd.Sub(timeStart))
		return true
	case endStamp := <-time.After(launchTimeout):
		logger.Info(cmdStr, "startup timed out!", endStamp.Sub(timeStart))
		return false
	}
}

func (m *SessionManager) launchWithoutWait(bin string, args ...string) {
	cmd := exec.Command(bin, args...)
	go cmd.Run()
}

func (m *SessionManager) launch(bin string, wait bool, args ...string) bool {
	if bin == "dde-session-daemon-part2" {
		return m.startSessionDaemonPart2()
	}

	if swapSchedDispatcher != nil {
		cgroupPath := swapSchedDispatcher.GetDECGroup()
		argsTemp := []string{"-g", "memory:" + cgroupPath, bin}
		args = append(argsTemp, args...)
		bin = globalCgExecBin
	}
	logger.Debugf("sessionManager.launch %q %v", bin, args)

	if wait {
		return m.launchWait(bin, args...)
	}
	m.launchWithoutWait(bin, args...)
	return true
}

func (m *SessionManager) AllowSessionDaemonRun() bool {
	return m.allowSessionDaemonRun
}

func (m *SessionManager) Register(id string) bool {
	m.cookieLocker.Lock()
	defer m.cookieLocker.Unlock()

	timeCh := m.cookies[id]
	if timeCh == nil {
		return false
	}
	delete(m.cookies, id)
	timeCh <- time.Now()
	return true
}
