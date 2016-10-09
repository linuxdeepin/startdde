package main

import (
	"fmt"
	// "os"
)

const (
	DPI_FALLBACK = 96
	HIDPI_LIMIT  = DPI_FALLBACK * 2
)

// TODO: update 'antialias, hinting, hintstyle, rgba, cursor-theme, cursor-size'
func (m *XSManager) updateDPI() {
	scale := m.gs.GetDouble("scale-factor")
	if scale == 0 {
		return
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

	updateXResources(xresourceInfos{
		&xresourceInfo{
			key:   "Xft.dpi",
			value: fmt.Sprintf("%v", int32(DPI_FALLBACK*scale)),
		},
	})
}
