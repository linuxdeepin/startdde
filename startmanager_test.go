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
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"pkg.deepin.io/lib/appinfo/desktopappinfo"
)

func _TestSetAutostart(t *testing.T) { //nolint
	m := StartManager{}
	if err := m.setAutostart("dropbox.desktop", true); err != nil {
		fmt.Println(err)
	}
	if !m.isAutostart("dropbox.desktop") {
		t.Error("set to autostart failed")
	}
	if err := m.setAutostart("dropbox.desktop", false); err != nil {
		fmt.Println(err)
	}
	if m.isAutostart("dropbox.desktop") {
		t.Error("set to not autostart failed")
	}
}

func _TestScanDir(t *testing.T) { //nolint
	scanDir("/tmp", func(p string, info os.FileInfo) bool {
		t.Log(info.Name())
		return false
	})
}

func Test_getLaunchedHooks(t *testing.T) {
	type args struct {
		hookDir string
	}
	tests := []struct {
		name    string
		args    args
		wantRet []string
	}{
		{
			name: "getLaunchedHooks",
			args: args{
				hookDir: "testdata/launched_hook",
			},
			wantRet: []string{
				"one",
				"two",
				"three",
			},
		},
		{
			name: "getLaunchedHooks_notexist",
			args: args{
				hookDir: "testdata/launched_hook.notexist",
			},
			wantRet: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRet := getLaunchedHooks(tt.args.hookDir)
			assert.ElementsMatch(t, tt.wantRet, gotRet)
		})
	}
}

func TestStartManager_getCpuFreqAdjustMap(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		obj  *StartManager
		args args
		want map[string]int32
	}{
		{
			name: "StartManager_getCpuFreqAdjustMap",
			obj:  &StartManager{},
			args: args{
				path: "testdata/cpuFreqAdjustFiles/app_startup.conf",
			},
			want: map[string]int32{
				"dde-calendar":           3,
				"dde-control-center":     3,
				"dde-file-manager":       3,
				"dde-printer":            3,
				"deepin-album":           3,
				"deepin-appstore":        3,
				"deepin-calculator":      3,
				"deepin-compressor":      3,
				"deepin-draw":            3,
				"deepin-image-viewer":    3,
				"deepin-movie":           3,
				"deepin-music":           3,
				"deepin-reader":          3,
				"deepin-screen-recorder": 3,
				"deepin-voice-note":      3,
				"dman":                   3,
				"uos-browser":            3,
			},
		},
		{
			name: "StartManager_getCpuFreqAdjustMap_notexist",
			obj:  &StartManager{},
			args: args{
				path: "testdata/cpuFreqAdjustFiles/app_startup.notexist",
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.obj.getCpuFreqAdjustMap(tt.args.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStartManager_enableCpuFreqLock(t *testing.T) {
	type args struct {
		desktopFile string
	}
	tests := []struct {
		name    string
		obj     *StartManager
		args    args
		wantErr bool
	}{
		{
			name: "StartManager_enableCpuFreqLock",
			obj: &StartManager{
				cpuFreqAdjustMap: map[string]int32{
					"dde-file-manager": 3,
					"dde-printer":      3,
					"deepin-album":     3,
				},
			},
			args: args{
				desktopFile: "not exist",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.obj.enableCpuFreqLock(tt.args.desktopFile)
			if tt.wantErr {
				assert.NotNil(t, err)
			}
		})
	}
}

func TestStartManager_getRestartTime(t *testing.T) {
	type args struct {
		appInfo *desktopappinfo.DesktopAppInfo
	}
	type test struct {
		name    string
		obj     *StartManager
		args    args
		want    time.Time
		wantSec bool
	}
	tests := []test{
		func() test {
			appInfo, _ := desktopappinfo.NewDesktopAppInfoFromFile("testdata/desktop/dde-file-manager.desktop")
			now := time.Now()
			return test{
				name: "StartManager_getRestartTime",
				obj: &StartManager{
					restartTimeMap: map[string]time.Time{appInfo.GetFileName(): now},
				},
				args: args{
					appInfo: appInfo,
				},
				want:    now,
				wantSec: true,
			}
		}(),
		func() test {
			appInfo, _ := desktopappinfo.NewDesktopAppInfoFromFile("testdata/desktop/dde-file-manager.desktop")
			return test{
				name: "StartManager_getRestartTime_notExist",
				obj:  &StartManager{},
				args: args{
					appInfo: appInfo,
				},
				want:    time.Time{},
				wantSec: false,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := tt.obj.getRestartTime(tt.args.appInfo)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantSec, got1)
		})
	}
}
