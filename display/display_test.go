/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
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

package display

import (
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMonitor(t *testing.T) {
	Convey("Monitor test", t, func() {
		var base = MonitorBaseInfo{
			Name:        "LVDS-1",
			UUID:        "xxxxlvds-1",
			Enabled:     true,
			X:           0,
			Y:           0,
			Width:       1377,
			Height:      768,
			Rotation:    1,
			Reflect:     0,
			RefreshRate: 60,
		}
		var lvds = &MonitorInfo{
			cfg:       &base,
			Name:      base.Name,
			Enabled:   true,
			Connected: true,
		}
		So(lvds.generateCommandline("LVDS-1", false), ShouldEqual, " --output LVDS-1 --primary --mode 1377x768 --pos 0x0 --rotate normal --reflect normal")
		So(lvds.generateCommandline("eDP-1", false), ShouldEqual, " --output LVDS-1 --mode 1377x768 --pos 0x0 --rotate normal --reflect normal")
		lvds.cfg.Enabled = false
		So(lvds.generateCommandline("LVDS-1", false), ShouldEqual, " --output LVDS-1 --off")
		lvds.cfg.Enabled = true
		lvds.Connected = false
		So(lvds.generateCommandline("LVDS-1", false), ShouldEqual, "")
		lvds.Connected = true

		var base1 = base
		base1.Name = "eDP-1"
		base1.UUID = "xxxxedp-1"
		var edp = &MonitorInfo{
			cfg:       &base1,
			Name:      base1.Name,
			Enabled:   true,
			Connected: true,
		}
		var infos = MonitorInfos{lvds, edp}
		So(infos.getByName("DVI-1"), ShouldBeNil)
		So(infos.getByName("LVDS-1").cfg.Name, ShouldEqual, "LVDS-1")
		So(infos.getMonitorsId(), ShouldEqual, "xxxxedp-1,xxxxlvds-1")
		So(infos.numberOfConnected(), ShouldEqual, 2)
		So(infos.canBePrimary("eDP-1").Name, ShouldEqual, "eDP-1")
		edp.Connected = false
		So(infos.numberOfConnected(), ShouldEqual, 1)
		So(infos.canBePrimary("eDP-1"), ShouldEqual, nil)
		So(infos.getMonitorsId(), ShouldEqual, "xxxxlvds-1")

		w, h := parseModeByRotation(1920, 1080, 1)
		So(w, ShouldEqual, 1920)
		So(h, ShouldEqual, 1080)
		w, h = parseModeByRotation(1920, 1080, 2)
		So(w, ShouldEqual, 1080)
		So(h, ShouldEqual, 1920)
	})
}

func TestConfigManager(t *testing.T) {
	Convey("Config manager test", t, func() {
		var monitor = configMonitor{
			Primary: "LVDS-1",
			BaseInfos: MonitorBaseInfos{
				{
					Name: "LVDS-1",
				},
			},
		}
		var manager = configManager{
			BaseGroup: make(map[string]*configMonitor),
			filename:  "config_tmp.json",
		}
		id := "xxxxlvds-1"
		So(manager.get(id), ShouldBeNil)
		manager.set(id, &monitor)
		So(manager.get(id).String(), ShouldEqual, monitor.String())
		So(manager.writeFile(), ShouldBeNil)
		t, err := newConfigManagerFromFile(manager.filename)
		So(err, ShouldBeNil)
		So(manager.String(), ShouldEqual, t.String())
		os.Remove(manager.filename)
		So(manager.delete(id), ShouldEqual, true)
		So(manager.delete(id), ShouldEqual, false)
		So(manager.get(id), ShouldBeNil)
	})
}

func TestConfigVersion(t *testing.T) {
	Convey("Test config version", t, func() {
		So(isVersionRight("3.0", "testdata/config.version"), ShouldEqual, true)
	})
}

func TestCalcRecommendedScaleFactor(t *testing.T) {
	Convey("Test calcRecommendedScaleFactor", t, func() {
		So(calcRecommendedScaleFactor(1366, 310), ShouldEqual, 1.0)
		So(calcRecommendedScaleFactor(1920, 527), ShouldEqual, 1.0)
		So(calcRecommendedScaleFactor(3840, 520), ShouldEqual, 1.75)
		So(calcRecommendedScaleFactor(1920, 294), ShouldEqual, 1.5)
		So(calcRecommendedScaleFactor(2160, 275), ShouldEqual, 1.75)
	})
}
