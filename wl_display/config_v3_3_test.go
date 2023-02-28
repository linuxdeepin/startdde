package display

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestConfig(t *testing.T) {
	var cfg0 ConfigV3_3
	err := json.Unmarshal([]byte(cfgStr0), &cfg0)
	require.NoError(t, err)

	cfg := cfg0.toConfig()
	assert.Len(t, cfg, 2)

	screenCfg := cfg["eDP12c5a5c24e4ab14126abf8dc36e7e9d4"]
	assert.NotNil(t, screenCfg)
	assert.Equal(t, &MonitorConfig{
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
		Primary:     true,
	}, screenCfg.Single)

	screenCfg = cfg["HDMIf5cae317c40b01139be5af61896be0cf,eDP12c5a5c24e4ab14126abf8dc36e7e9d4"]
	assert.NotNil(t, screenCfg)
	assert.Len(t, screenCfg.Custom, 1)
	customModeCfg := screenCfg.Custom[0]
	assert.Equal(t, "_dde_display_config_private", customModeCfg.Name)
	assert.Len(t, customModeCfg.Monitors, 2)
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
		Primary:     true,
	}, customModeCfg.Monitors[0])
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
		Primary:     false,
	}, customModeCfg.Monitors[1])
}
