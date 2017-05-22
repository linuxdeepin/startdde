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
	"os/exec"
	"pkg.deepin.io/dde/api/soundutils"
)

var objSoundThemePlayer *soundthemeplayer.SoundThemePlayer

func playLoginSound() {
	logger.Info("PlaySystemSound EventLogin")
	err := soundutils.PlaySystemSound(soundutils.EventLogin, "", true)
	if err != nil {
		logger.Warning("PlaySystemSound EventLogin failed:", err)
	}
	logger.Info("PlaySystemSound EventLogin done")
}

func initObjSoundThemePlayer() {
	var err error
	objSoundThemePlayer, err = soundthemeplayer.NewSoundThemePlayer(
		"com.deepin.api.SoundThemePlayer",
		"/com/deepin/api/SoundThemePlayer",
	)
	if err != nil {
		logger.Warning("New SoundThemePlayer failed:", err)
	}
}

func soundThemePlayerPlay(theme, event string) {
	if objSoundThemePlayer == nil {
		logger.Warning("Play sound theme failed: soundThemePlayer is nil")
		return
	}
	err := objSoundThemePlayer.Play(theme, event, soundutils.GetSoundPlayer())
	if err != nil {
		logger.Warningf("Play sound theme failed: theme %q, event %q, error: %v", theme, event, err)
	}
}

func quitSoundThemePlayer() {
	if objSoundThemePlayer == nil {
		logger.Warning("quitSoundThemePlayer failed: soundThemePlayer is nil")
		return
	}
	objSoundThemePlayer.Quit()
}

func quitPulseAudio() {
	logger.Debug("quit pulse audio")
	out, err := exec.Command("/usr/bin/pulseaudio", "--kill").CombinedOutput()
	if err != nil {
		logger.Error("quit pulseaudio failed:", string(out))
	}
}

func preparePlayShutdownSound() {
	err := soundutils.SetShutdownSound(
		soundutils.CanPlayEvent(),
		soundutils.GetSoundTheme(),
		soundutils.EventShutdown)
	if err != nil {
		logger.Warning("Set shutdown sound failed:", err)
	}
}
