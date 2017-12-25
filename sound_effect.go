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
	"errors"
	"fmt"
	"os/exec"

	"dbus/com/deepin/api/soundthemeplayer"

	"pkg.deepin.io/dde/api/soundutils"
	"pkg.deepin.io/lib/pulse"
)

var soundThemePlayer *soundthemeplayer.SoundThemePlayer

func playLoginSound() {
	logger.Info("PlaySystemSound DesktopLogin")
	err := soundutils.PlaySystemSound(soundutils.EventDesktopLogin, "")
	if err != nil {
		logger.Warning("PlaySystemSound DesktopLogin failed:", err)
	}
	logger.Info("PlaySystemSound DesktopLogin done")
}

func playLogoutSound() {
	device, err := getALSADevice()
	if err != nil {
		logger.Warning("failed to get ALSA device:", err)
		return
	}
	logger.Debugf("ALSA device: %q", device)
	quitPulseAudio()
	err = soundThemePlayer.Play(soundutils.GetSoundTheme(),
		soundutils.EventDesktopLogout, device)
	if err != nil {
		logger.Warning("SoundThemePlayer.Play err:", err)
	}
}

func initSoundThemePlayer() {
	var err error
	soundThemePlayer, err = soundthemeplayer.NewSoundThemePlayer(
		"com.deepin.api.SoundThemePlayer",
		"/com/deepin/api/SoundThemePlayer",
	)

	if err != nil {
		panic(fmt.Errorf("NewSoundThemePlayer err: %v", err))
	}
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

func getALSADevice() (string, error) {
	ctx := pulse.GetContext()
	if ctx == nil {
		return "", errors.New("failed to get pulse context")
	}
	defer ctx.Free()

	defaultSinkName := ctx.GetDefaultSink()
	var defaultSink *pulse.Sink
	for _, sink := range ctx.GetSinkList() {
		if sink.Name == defaultSinkName {
			defaultSink = sink
			break
		}
	}

	if defaultSink == nil {
		return "", errors.New("failed to get default sink")
	}

	props := defaultSink.PropList
	card := props["alsa.card"]
	device := props["alsa.device"]
	if card == "" || device == "" {
		return "", errors.New("failed to get sink ALSA property")
	}
	deviceStr := fmt.Sprintf("plughw:CARD=%s,DEV=%s", card, device)

	subdevice := props["alsa.subdevice"]
	if subdevice != "" {
		deviceStr = deviceStr + ",SUBDEV=" + subdevice
	}
	return deviceStr, nil
}
