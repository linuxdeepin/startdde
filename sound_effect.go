// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	dbus1 "github.com/godbus/dbus"
	"github.com/linuxdeepin/dde-api/soundutils"
	soundthemeplayer "github.com/linuxdeepin/go-dbus-factory/com.deepin.api.soundthemeplayer"
	"github.com/linuxdeepin/go-lib/pulse"
)

var soundThemePlayer soundthemeplayer.SoundThemePlayer

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
		quitPulseAudio()
		return
	}

	if mute {
		logger.Debug("default sink is mute")
		quitPulseAudio()
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

const (
	audioInterface   = "com.deepin.daemon.Audio"
	audioServiceName = audioInterface
	audioPath        = "/com/deepin/daemon/Audio"
)

func startPulseAudio() error {
	err := exec.Command("systemctl", "--user", "--runtime", "unmask", "pulseaudio.service").Run()
	if err != nil {
		return err
	}
	err = exec.Command("systemctl", "--user", "start", "pulseaudio.service").Run()
	if err != nil {
		return err
	}

	return nil
}

func quitPulseAudio() {
	logger.Debug("quit pulse audio")

	sessionBus, err := dbus1.SessionBus()
	if err != nil {
		logger.Warning("failed to get session bus:", err)
	} else {
		audioObj := sessionBus.Object(audioServiceName, audioPath)
		err = audioObj.Call(audioInterface+".NoRestartPulseAudio", dbus1.FlagNoAutoStart).Err
		if err != nil {
			logger.Warning("failed to call NoRestartPulseAudio:", err)
		}
	}

	// mask pulseaudio.service
	out, err := exec.Command("systemctl", "--user", "--runtime", "--now", "mask",
		"pulseaudio.service").CombinedOutput()
	if err != nil {
		logger.Warningf("temp mask pulseaudio.service failed err: %v, out: %s", err, out)
	}

	// view status
	err = exec.Command("systemctl", "--quiet", "--user", "is-active",
		"pulseaudio.service").Run()
	if err == nil {
		logger.Warning("pulseaudio.service is still running")
	} else {
		logger.Debug("pulseaudio.service is stopped, err:", err)
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
