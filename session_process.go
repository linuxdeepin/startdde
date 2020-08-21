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

	"crypto/rand"
	"io"
	"os"
	"os/exec"
	"time"

	dbus "github.com/godbus/dbus"
)

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

// 如果 endFn 为 nil，则等待命令完成或结束；如果 endFn 不为 nil，则不等待，命令行启动后就返回，命令完成或结束后调用 endFn。
func (m *SessionManager) launchWaitAux(cookie, program string, args []string, cmdWaitDelay time.Duration, endFn func(bool)) (launchOk bool) {

	cmd := exec.Command(program, args...)
	cmd.Env = append(os.Environ(), "DDE_SESSION_PROCESS_COOKIE_ID="+cookie)

	ch := make(chan time.Time, 1)
	m.cookieLocker.Lock()
	m.cookies[cookie] = ch
	m.cookieLocker.Unlock()

	cmdStr := fmt.Sprintf("%s %v", program, args)
	timeStart := time.Now()
	err := cmd.Start()
	if err != nil {
		logger.Warningf("start command %s failed: %v", cmdStr, err)
		if endFn != nil {
			endFn(launchOk)
		}
		return false
	}
	logger.Infof("command %s started, pid: %v", cmdStr, cmd.Process.Pid)

	time.AfterFunc(cmdWaitDelay, func() {
		err := cmd.Wait()
		if err != nil {
			logger.Warningf("command %s exit with error: %v", cmdStr, err)
		}
		m.cookieLocker.Lock()
		ch := m.cookies[cookie]
		if ch != nil {
			delete(m.cookies, cookie)
			ch <- time.Now()
		}
		m.cookieLocker.Unlock()
	})

	waitCh := func() {
		select {
		case timeEnd := <-ch:
			logger.Info(cmdStr, "startup duration:", timeEnd.Sub(timeStart))
			launchOk = true
		case timeEnd := <-time.After(launchTimeout):
			logger.Info(cmdStr, "startup timed out!", timeEnd.Sub(timeStart))
		}
	}

	if endFn != nil {
		go func() {
			waitCh()
			endFn(launchOk)
		}()
	} else {
		waitCh()
	}
	return
}

func (m *SessionManager) launchWaitCore(name string, program string, args []string, cmdWaitDelay time.Duration, endFn func(bool)) {
	m.launchWaitAux(name, program, args, cmdWaitDelay, endFn)
}

func (m *SessionManager) launchWait(program string, args ...string) bool {
	cookie := genUuid()
	return m.launchWaitAux(cookie, program, args, 0, nil)
}

func (m *SessionManager) launchWithoutWait(bin string, args ...string) {
	cmd := exec.Command(bin, args...)
	go func() {
		err := cmd.Run()
		if err != nil {
			logger.Warning(err)
		}
	}()
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

func (m *SessionManager) AllowSessionDaemonRun() (bool, *dbus.Error) {
	return m.allowSessionDaemonRun, nil
}

func (m *SessionManager) Register(id string) (bool, *dbus.Error) {
	m.cookieLocker.Lock()
	defer m.cookieLocker.Unlock()

	timeCh := m.cookies[id]
	if timeCh == nil {
		return false, nil
	}
	delete(m.cookies, id)
	timeCh <- time.Now()
	return true, nil
}
