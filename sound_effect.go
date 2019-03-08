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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/linuxdeepin/go-dbus-factory/com.deepin.api.soundthemeplayer"
	"pkg.deepin.io/dde/api/soundutils"
	dbus1 "pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/pulse"
)

var soundThemePlayer *soundthemeplayer.SoundThemePlayer

func playLoginSound() {
	markFile := filepath.Join(os.TempDir(), "startdde-login-sound-mark")
	_, err := os.Stat(markFile)
	if err == nil {
		// already played
		return
	} else if !os.IsNotExist(err) {
		logger.Warning(err)
	}

	defer func() {
		err := ioutil.WriteFile(markFile, nil, 0644)
		if err != nil {
			logger.Warning(err)
		}
	}()

	autoLoginUser, err := getLightDMAutoLoginUser()
	if err != nil {
		logger.Warning(err)
		return
	}

	u, err := user.Current()
	if err != nil {
		logger.Warning(err)
		return
	}

	if u.Username != autoLoginUser {
		return
	}

	logger.Info("PlaySystemSound DesktopLogin")
	err = soundutils.PlaySystemSound(soundutils.EventDesktopLogin, "")
	if err != nil {
		logger.Warning("PlaySystemSound DesktopLogin failed:", err)
	}
	logger.Info("PlaySystemSound DesktopLogin done")
}

func getDefaultSinkAlsaDevice() (device string, mute bool, err error) {
	ctx := pulse.GetContext()
	if ctx == nil {
		err = errors.New("failed to get pulse.Context")
		return
	}

	defaultSink := getPulseDefaultSink(ctx)
	if defaultSink == nil {
		err = errors.New("failed to get default sink")
		return
	}

	if defaultSink.Mute {
		mute = true
		return
	}

	device, err = getSinkAlsaDevice(defaultSink)
	return
}

func playLogoutSound() {
	device, mute, err := getDefaultSinkAlsaDevice()
	if err != nil {
		logger.Warning(err)
		return
	}

	if mute {
		logger.Debug("default sink is mute")
		return
	}
	logger.Debugf("ALSA device: %q", device)
	quitPulseAudio()
	err = soundThemePlayer.Play(0, soundutils.GetSoundTheme(),
		soundutils.EventDesktopLogout, device)
	if err != nil {
		logger.Warning("SoundThemePlayer.Play err:", err)
	}
}

func initSoundThemePlayer() {
	sysBus, err := dbus1.SystemBus()
	if err != nil {
		return
	}

	soundThemePlayer = soundthemeplayer.NewSoundThemePlayer(sysBus)
}

func quitPulseAudio() {
	logger.Debug("quit pulse audio")
	out, err := exec.Command("/usr/bin/pulseaudio", "--kill").CombinedOutput()
	if err != nil {
		logger.Error("quit pulseaudio failed:", string(out))
	}
}

func preparePlayShutdownSound() {
	canPlay := soundutils.CanPlayEvent(soundutils.EventSystemShutdown)
	var device string
	if canPlay {
		var mute bool
		var err error
		device, mute, err = getDefaultSinkAlsaDevice()
		if err != nil {
			logger.Warning(err)
			return
		}

		if mute {
			logger.Debug("default sink is mute")
			canPlay = false
		}
	}

	var cfg soundutils.ShutdownSoundConfig
	if canPlay {
		cfg = soundutils.ShutdownSoundConfig{
			CanPlay: canPlay,
			Theme:   soundutils.GetSoundTheme(),
			Event:   soundutils.EventSystemShutdown,
			Device:  device,
		}
	}

	logger.Debugf("set shutdown sound config: %+v", cfg)
	err := soundutils.SetShutdownSoundConfig(&cfg)
	if err != nil {
		logger.Warning("failed to set shutdown sound config:", err)
	}
}

func getPulseDefaultSink(ctx *pulse.Context) (defaultSink *pulse.Sink) {
	defaultSinkName := ctx.GetDefaultSink()
	for _, sink := range ctx.GetSinkList() {
		if sink.Name == defaultSinkName {
			defaultSink = sink
			break
		}
	}
	return
}

func getSinkAlsaDevice(sink *pulse.Sink) (string, error) {
	props := sink.PropList
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
