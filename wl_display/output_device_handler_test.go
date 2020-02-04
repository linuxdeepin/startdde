package display

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getMaxAreaOutputDeviceMode(t *testing.T) {
	maxAreaMode := getMaxAreaOutputDeviceMode([]outputDeviceMode{
		{
			Width:  1024,
			Height: 768,
			ModeId: 0,
		},
		{
			Width:  640,
			Height: 480,
			ModeId: 1,
		},
		{
			Width:  1280,
			Height: 720,
			ModeId: 2,
		},
		{
			Width:  800,
			Height: 600,
			ModeId: 3,
		},
	})
	assert.Equal(t, maxAreaMode.ModeId, int32(2))

	maxAreaMode = getMaxAreaOutputDeviceMode([]outputDeviceMode{
		{
			Width:  1024,
			Height: 768,
			ModeId: 1,
		}})
	assert.Equal(t, maxAreaMode.ModeId, int32(1))
}
