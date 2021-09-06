package display

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

const cfgStr1 = `{
  "HDMI-08007177627d2ab1ae109501a57fb9178,VGA-06187f7cabc416d9a3f2658bf7c66695a": {
    "Custom": [
      {
        "Name": "_dde_display_config_private",
        "Monitors": [
          {
            "UUID": "HDMI-08007177627d2ab1ae109501a57fb9178",
            "Name": "HDMI-0",
            "Enabled": true,
            "X": 0,
            "Y": 0,
            "Width": 1920,
            "Height": 1080,
            "Rotation": 1,
            "Reflect": 0,
            "RefreshRate": 60,
            "Primary": true
          },
          {
            "UUID": "VGA-06187f7cabc416d9a3f2658bf7c66695a",
            "Name": "VGA-0",
            "Enabled": true,
            "X": 0,
            "Y": 0,
            "Width": 1920,
            "Height": 1080,
            "Rotation": 1,
            "Reflect": 0,
            "RefreshRate": 60,
            "Primary": false
          }
        ]
      }
    ]
  }
}
`

func TestConfig_v4(t *testing.T) {
	m := &Manager{
		DisplayMode: DisplayModeExtend,
		Brightness: map[string]float64{"HDMI-0" : 0.59, "VGA-0" : 0.59},
		gsColorTemperatureMode: 0,
		gsColorTemperatureManual: 6500,
	}
	var cfg0 ConfigV4
	err := json.Unmarshal([]byte(cfgStr1), &cfg0)
	require.Nil(t, err)

	cfg := cfg0.toConfig(m)
	assert.Len(t, cfg, 1)

	screenCfg := cfg["HDMI-08007177627d2ab1ae109501a57fb9178,VGA-06187f7cabc416d9a3f2658bf7c66695a"]
	assert.NotNil(t, screenCfg)
	mirrorModeCfg := screenCfg.Mirror
	assert.Len(t, mirrorModeCfg.Monitors, 2)
	assert.Equal(t, &MonitorConfig{
		UUID:        "HDMI-08007177627d2ab1ae109501a57fb9178",
		Name:        "HDMI-0",
		Enabled:     true,
		X:           0,
		Y:           0,
		Width:       1920,
		Height:      1080,
		Rotation:    1,
		Reflect:     0,
		RefreshRate: 60,
		Brightness:  0.59,
		Primary:     true,
		ColorTemperatureMode: 0,
		ColorTemperatureManual: 6500,
	}, mirrorModeCfg.Monitors[0])
	assert.Equal(t, &MonitorConfig{
		UUID:        "VGA-06187f7cabc416d9a3f2658bf7c66695a",
		Name:        "VGA-0",
		Enabled:     true,
		X:           0,
		Y:           0,
		Width:       1920,
		Height:      1080,
		Rotation:    1,
		Reflect:     0,
		RefreshRate: 60,
		Brightness:  0.59,
		Primary:     false,
		ColorTemperatureMode: 0,
		ColorTemperatureManual: 6500,
	}, mirrorModeCfg.Monitors[1])
}
