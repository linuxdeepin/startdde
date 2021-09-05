package display

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	configPath_v3     = "./testdata/display_v3.json"
	configPath_v4     = "./testdata/display_v4.json"
	configPath_v5     = "./testdata/display_v5.json"
	builtinMonitorCfg = "./testdata/builtin-monitor"
)

func TestConfig(t *testing.T) {
	_, err := loadConfigV3_3(configPath_v3)
	require.Nil(t, err)

	_, err = loadConfigV4(configPath_v4)
	require.Nil(t, err)

	config, err := loadConfigV5(configPath_v5)
	require.Nil(t, err)

	screenconfig := config["HDMI-1bc06f293ee6bfb16fd813648741f8ac3,eDP-12fd580d2dc41168dce2efb1bf19adb54"]
	require.NotNil(t, screenconfig)

	modeconfig := screenconfig.getModeConfigs(DisplayModeExtend)
	require.NotNil(t, modeconfig)

	monitors := modeconfig.Monitors
	require.NotEqual(t, len(monitors), 0)

	monitorconfig := getMonitorConfigByUuid(monitors, "eDP-12fd580d2dc41168dce2efb1bf19adb54")
	require.NotNil(t, monitorconfig)

	primarymonitor := getMonitorConfigPrimary(monitors)
	require.Equal(t, primarymonitor.UUID, monitorconfig.UUID)

	setMonitorConfigsPrimary(monitors, "HDMI-1bc06f293ee6bfb16fd813648741f8ac3")
	primarymonitor = getMonitorConfigPrimary(monitors)
	require.Equal(t, primarymonitor.UUID, "HDMI-1bc06f293ee6bfb16fd813648741f8ac3")

	_, err = loadBuiltinMonitorConfig(builtinMonitorCfg)
	require.Nil(t, err)

	err = saveBuiltinMonitorConfig(builtinMonitorCfg, "eDP-1")
	require.Nil(t, err)
}
