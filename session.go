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
	"dbus/com/deepin/api/soundthemeplayer"
	"dbus/org/freedesktop/login1"
	"fmt"
	"os"
	"sync"
	"time"

	"pkg.deepin.io/dde/api/soundutils"
	"pkg.deepin.io/lib/dbus"
)

type SessionManager struct {
	CurrentUid string
	cookies    map[string]chan time.Time
	Stage      int32
}

const (
	cmdLock     = "/usr/bin/dde-lock"
	cmdShutdown = "/usr/bin/dde-shutdown"
)

const (
	SessionStageInitBegin int32 = iota
	SessionStageInitEnd
	SessionStageCoreBegin
	SessionStageCoreEnd
	SessionStageAppsBegin
	SessionStageAppsEnd
)

var (
	objLogin *login1.Manager
)

func (m *SessionManager) CanLogout() bool {
	return true
}

func (m *SessionManager) Logout() {
	m.launch(cmdShutdown, false)
}

func (m *SessionManager) RequestLogout() {
	if soundutils.CanPlayEvent() {
		// Try to launch 'sound-theme-player'
		playThemeSound("", "")
		// Play sound
		playThemeSound(soundutils.GetSoundTheme(),
			soundutils.EventLogout)
	}
	os.Exit(0)
}

func (m *SessionManager) ForceLogout() {
	os.Exit(0)
}

func (shudown *SessionManager) CanShutdown() bool {
	str, _ := objLogin.CanPowerOff()
	if str == "yes" {
		return true
	}

	return false
}

func (m *SessionManager) Shutdown() {
	m.launch(cmdShutdown, false)
}

func (m *SessionManager) RequestShutdown() {
	err := soundutils.SetShutdownSound(
		soundutils.CanPlayEvent(),
		soundutils.GetSoundTheme(),
		soundutils.EventShutdown)
	if err != nil {
		logger.Warning("Set shutdown sound failed:", err)
	}
	objLogin.PowerOff(true)
}

func (m *SessionManager) ForceShutdown() {
	objLogin.PowerOff(false)
}

func (shudown *SessionManager) CanReboot() bool {
	str, _ := objLogin.CanReboot()
	if str == "yes" {
		return true
	}

	return false
}

func (m *SessionManager) Reboot() {
	m.launch(cmdShutdown, false)
}

func (m *SessionManager) RequestReboot() {
	objLogin.Reboot(true)
}

func (m *SessionManager) ForceReboot() {
	objLogin.Reboot(false)
}

func (m *SessionManager) CanSuspend() bool {
	str, _ := objLogin.CanSuspend()
	if str == "yes" {
		return true
	}
	return false
}

func (m *SessionManager) RequestSuspend() {
	objLogin.Suspend(false)
}

func (m *SessionManager) CanHibernate() bool {
	str, _ := objLogin.CanHibernate()
	if str == "yes" {
		return true
	}
	return false
}

func (m *SessionManager) RequestHibernate() {
	objLogin.Hibernate(false)
}

func (m *SessionManager) RequestLock() error {
	m.launch(cmdLock, false)
	return nil
}

func (m *SessionManager) PowerOffChoose() {
	m.launch(cmdShutdown, false)
}

func initSession() {
	var err error

	objLogin, err = login1.NewManager("org.freedesktop.login1",
		"/org/freedesktop/login1")
	if err != nil {
		panic(fmt.Errorf("New Login1 Failed: %s", err))
	}
}

func newSessionManager() *SessionManager {
	m := &SessionManager{}
	m.cookies = make(map[string]chan time.Time)
	m.setPropName("CurrentUid")

	return m
}
func (manager *SessionManager) launchWindowManager() {
	manager.launch(*windowManagerBin, false)
}

func startSession() {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("StartSession recover:", err)
			return
		}
	}()

	initSession()

	// Fixed: Set `GNOME_DESKTOP_SESSION_ID` to cheat `xdg-open`
	// https://tower.im/projects/8162ac3745044ca29f9f3d21beaeb93d/todos/d51f8f2a317740cca3af15384d34e79f/
	os.Setenv("GNOME_DESKTOP_SESSION_ID", "this-is-deprecated")
	os.Setenv("XDG_CURRENT_DESKTOP", "Deepin")

	manager := newSessionManager()
	err := dbus.InstallOnSession(manager)
	if err != nil {
		logger.Error("Install Session DBus Failed:", err)
		return
	}

	manager.setPropStage(SessionStageInitBegin)
	manager.launchWindowManager()
	manager.setPropStage(SessionStageInitEnd)

	manager.setPropStage(SessionStageCoreBegin)
	startStartManager()

	// dde-launcher is fast enough now, there's no need to start it at the begnning
	// of every session.
	// manager.launch("/usr/bin/dde-launcher", false, "--hidden")

	var wg sync.WaitGroup
	wg.Add(2)

	// dde-desktop and dde-dock-trash-plugin reply on deepin-file-manager-backend
	// to run properly.
	go func() {
		manager.launch("/usr/lib/deepin-daemon/deepin-file-manager-backend", true)
		manager.launch("/usr/bin/dde-desktop", true)
		wg.Done()
	}()

	go func() {
		manager.launch("/usr/lib/deepin-daemon/dde-preload", true)
		manager.launch("/usr/bin/dde-dock", true)
		manager.launch("/usr/lib/deepin-daemon/dde-session-daemon", false)
		wg.Done()
	}()
	wg.Wait()

	manager.setPropStage(SessionStageCoreEnd)

	manager.setPropStage(SessionStageAppsBegin)

	if !*debug {
		startAutostartProgram()
	}
	manager.setPropStage(SessionStageAppsEnd)
}

var themePlayer *soundthemeplayer.SoundThemePlayer

func playThemeSound(theme, event string) {
	if themePlayer == nil {
		var err error
		themePlayer, err = soundthemeplayer.NewSoundThemePlayer(
			"com.deepin.api.SoundThemePlayer",
			"/com/deepin/api/SoundThemePlayer",
		)
		if err != nil {
			logger.Error("Init 'SoundThemePlayer' failed:", err)
			return
		}
	}

	err := themePlayer.Play(theme, event)
	if err != nil {
		logger.Error("Play sound theme failed:", theme, event, err)
	}
}
