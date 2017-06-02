package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"pkg.deepin.io/lib/utils"
	"strings"
)

const (
	DPI_FALLBACK = 96
	HIDPI_LIMIT  = DPI_FALLBACK * 2

	ffKeyPixels = `user_pref("layout.css.devPixelsPerPx",`
)

// TODO: update 'antialias, hinting, hintstyle, rgba, cursor-theme, cursor-size'
func (m *XSManager) updateDPI() {
	scale := m.gs.GetDouble("scale-factor")
	if scale <= 0 {
		scale = 1
	}

	// TODO: update QT DPI
	// QT_SCALE_FACTOR will cause dde-dock not show
	// os.Setenv("QT_SCALE_FACTOR", fmt.Sprintf("%v", scale))

	scaledDPI := int32(float64(DPI_FALLBACK*1024) * scale)
	if scaledDPI == m.gs.GetInt("xft-dpi") {
		return
	}

	m.gs.SetInt("xft-dpi", scaledDPI)
	m.setSettings([]xsSetting{
		{
			sType: settingTypeInteger,
			prop:  "Xft/DPI",
			value: scaledDPI,
		},
	})

}

func (m *XSManager) updateXResources() {
	scale := m.gs.GetDouble("scale-factor")
	if scale <= 0 {
		scale = 1
	}
	updateXResources(xresourceInfos{
		&xresourceInfo{
			key:   "Xcursor.theme",
			value: m.gs.GetString("gtk-cursor-theme-name"),
		},
		&xresourceInfo{
			key:   "Xcursor.size",
			value: fmt.Sprintf("%d", m.gs.GetInt("gtk-cursor-theme-size")),
		},
		&xresourceInfo{
			key:   "Xft.dpi",
			value: fmt.Sprintf("%v", int32(DPI_FALLBACK*scale)),
		},
	})
}

var ffDir = path.Join(os.Getenv("HOME"), ".mozilla/firefox")

func (m *XSManager) updateFirefoxDPI() {
	scale := m.gs.GetDouble("scale-factor")
	if scale <= 0 {
		// firefox default value: -1
		scale = -1
	}

	configs, err := getFirefoxConfigs(ffDir)
	if err != nil {
		logger.Debug("Failed to get firefox configs:", err)
		return
	}

	for _, config := range configs {
		err = setFirefoxDPI(scale, config, config)
		if err != nil {
			logger.Warning("Failed to set firefox dpi:", config, err)
		}
	}
}

func getFirefoxConfigs(dir string) ([]string, error) {
	finfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var configs []string
	for _, finfo := range finfos {
		config := path.Join(dir, finfo.Name(), "prefs.js")
		if !utils.IsFileExist(config) {
			continue
		}
		configs = append(configs, config)
	}
	return configs, nil
}

func setFirefoxDPI(value float64, src, dest string) error {
	contents, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	lines := strings.Split(string(contents), "\n")
	target := fmt.Sprintf("%s \"%.2f\");", ffKeyPixels, value)
	found := false
	for i, line := range lines {
		if line == "" || line[0] == '#' {
			continue
		}
		if !strings.Contains(line, ffKeyPixels) {
			continue
		}

		if line == target {
			return nil
		}

		tmp := strings.Split(ffKeyPixels, ",")[0] + ", " +
			fmt.Sprintf("\"%.2f\");", value)
		lines[i] = tmp
		found = true
		break
	}
	if !found {
		if value == -1 {
			return nil
		}
		tmp := lines[len(lines)-1]
		lines[len(lines)-1] = target
		lines = append(lines, tmp)
	}
	return ioutil.WriteFile(dest, []byte(strings.Join(lines, "\n")), 0644)
}
