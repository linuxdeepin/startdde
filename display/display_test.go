package display

import (
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"testing"
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
		So(lvds.generateCommandline("LVDS-1", false), ShouldEqual, " --output LVDS-1 --primary --mode 1377x768 --rate 60.00 --pos 0x0 --scale 1x1 --rotate normal --reflect normal")
		So(lvds.generateCommandline("eDP-1", false), ShouldEqual, " --output LVDS-1 --mode 1377x768 --rate 60.00 --pos 0x0 --scale 1x1 --rotate normal --reflect normal")
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
		So(infos.getMonitorsId(), ShouldEqual, "xxxxlvds-1,xxxxedp-1")
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
