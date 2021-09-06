package display

import (
"encoding/json"
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
"testing"
)

const cfgStr0 = `{
  "eDP12c5a5c24e4ab14126abf8dc36e7e9d4": {
    "Name": "",
    "Primary": "eDP-1",
    "BaseInfos": [
      {
        "UUID": "eDP12c5a5c24e4ab14126abf8dc36e7e9d4",
        "Name": "eDP-1",
        "Enabled": true,
        "X": 0,
        "Y": 0,
        "Width": 1360,
        "Height": 768,
        "Rotation": 1,
        "Reflect": 0,
        "RefreshRate": 59.79899072004335,
        "MmWidth": 0,
        "MmHeight": 0
      }
    ]
  },
  "_dde_display_config_private+HDMIf5cae317c40b01139be5af61896be0cf,eDP12c5a5c24e4ab14126abf8dc36e7e9d4": {
    "Name": "_dde_display_config_private",
    "Primary": "eDP-1",
    "BaseInfos": [
      {
        "UUID": "eDP12c5a5c24e4ab14126abf8dc36e7e9d4",
        "Name": "eDP-1",
        "Enabled": true,
        "X": 0,
        "Y": 0,
        "Width": 1366,
        "Height": 768,
        "Rotation": 1,
        "Reflect": 0,
        "RefreshRate": 60.00471735199308,
        "MmWidth": 0,
        "MmHeight": 0
      },
      {
        "UUID": "HDMIf5cae317c40b01139be5af61896be0cf",
        "Name": "HDMI-2",
        "Enabled": true,
        "X": 1366,
        "Y": 0,
        "Width": 1920,
        "Height": 1080,
        "Rotation": 1,
        "Reflect": 0,
        "RefreshRate": 60,
        "MmWidth": 0,
        "MmHeight": 0
      }
    ]
  }
}
`

func TestConfig_v3(t *testing.T) {
	m := &Manager{
		DisplayMode: DisplayModeExtend,
		Brightness: map[string]float64{"HDMI-2" : 0.59, "eDP-1" : 0.59},
		gsColorTemperatureMode: 0,
		gsColorTemperatureManual: 6500,
	}
	var cfg0 ConfigV3_3
	err := json.Unmarshal([]byte(cfgStr0), &cfg0)
	require.Nil(t, err)

	cfg := cfg0.toConfig(m)
	assert.Len(t, cfg, 2)

	screenCfg := cfg["eDP12c5a5c24e4ab14126abf8dc36e7e9d4"]
	assert.NotNil(t, screenCfg)
	assert.Equal(t, &SingleModeConfig{
		Monitor: &MonitorConfig{
			UUID:        "eDP12c5a5c24e4ab14126abf8dc36e7e9d4",
			Name:        "eDP-1",
			Enabled:     true,
			X:           0,
			Y:           0,
			Width:       1360,
			Height:      768,
			Rotation:    1,
			Reflect:     0,
			RefreshRate: 59.79899072004335,
			Brightness:  0.59,
			Primary:     true,
			ColorTemperatureMode:   0,
			ColorTemperatureManual: 0,
		},
		ColorTemperatureMode:   0,
		ColorTemperatureManual: 6500,
	} , screenCfg.Single)

	screenCfg = cfg["HDMIf5cae317c40b01139be5af61896be0cf,eDP12c5a5c24e4ab14126abf8dc36e7e9d4"]
	assert.NotNil(t, screenCfg)
	extendModeCfg := screenCfg.Extend
	assert.Len(t, extendModeCfg.Monitors, 2)
	assert.Equal(t, &MonitorConfig{
		UUID:        "eDP12c5a5c24e4ab14126abf8dc36e7e9d4",
		Name:        "eDP-1",
		Enabled:     true,
		X:           0,
		Y:           0,
		Width:       1366,
		Height:      768,
		Rotation:    1,
		Reflect:     0,
		RefreshRate: 60.00471735199308,
		Brightness:  0.59,
		Primary:     true,
		ColorTemperatureMode:   0,
		ColorTemperatureManual: 6500,
	}, extendModeCfg.Monitors[0])
	assert.Equal(t, &MonitorConfig{
		UUID:        "HDMIf5cae317c40b01139be5af61896be0cf",
		Name:        "HDMI-2",
		Enabled:     true,
		X:           1366,
		Y:           0,
		Width:       1920,
		Height:      1080,
		Rotation:    1,
		Reflect:     0,
		RefreshRate: 60,
		Brightness:  0.59,
		Primary:     false,
		ColorTemperatureMode:   0,
		ColorTemperatureManual: 6500,
	}, extendModeCfg.Monitors[1])
}