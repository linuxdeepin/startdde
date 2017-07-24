/*
 * Copyright (C) 2014 ~ 2017 Deepin Technology Co., Ltd.
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
	"dbus/com/deepin/api/soundthemeplayer"
	"os/exec"
	"pkg.deepin.io/dde/api/soundutils"
)

var objSoundThemePlayer *soundthemeplayer.SoundThemePlayer

func playLoginSound() {
	logger.Info("PlaySystemSound DesktopLogin")
	err := soundutils.PlaySystemSound(soundutils.EventDesktopLogin, "", true)
	if err != nil {
		logger.Warning("PlaySystemSound DesktopLogin failed:", err)
	}
	logger.Info("PlaySystemSound DesktopLogin done")
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
		soundutils.CanPlayEvent(soundutils.EventSystemShutdown),
		soundutils.GetSoundTheme(),
		soundutils.EventSystemShutdown)
	if err != nil {
		logger.Warning("Set shutdown sound failed:", err)
	}
}
