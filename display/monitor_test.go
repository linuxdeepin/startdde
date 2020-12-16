package display

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getFirstModeBySize(t *testing.T) {
	modes := []ModeInfo{{84, "", 1920, 1080, 60.0}, {85, "", 1920, 1080, 50.0},
		{95, "", 1600, 1200, 60.0}}
	assert.Equal(t, getFirstModeBySize(modes, 1920, 1080), ModeInfo{84, "", 1920, 1080, 60.0})
	assert.Equal(t, getFirstModeBySize(modes, 1600, 1200), ModeInfo{95, "", 1600, 1200, 60.0})
	assert.Equal(t, getFirstModeBySize(modes, 1280, 740), ModeInfo{})
}

func Test_getFirstModeBySizeRate(t *testing.T) {
	modes := []ModeInfo{{84, "", 1920, 1080, 60.0}, {85, "", 1920, 1080, 50.0},
		{95, "", 1600, 1200, 60.0}}
	assert.Equal(t, getFirstModeBySizeRate(modes, 1920, 1080, 59.99), ModeInfo{84, "", 1920, 1080, 60.0})
	assert.Equal(t, getFirstModeBySizeRate(modes, 1920, 1080, 49.99), ModeInfo{85, "", 1920, 1080, 50.0})
	assert.Equal(t, getFirstModeBySizeRate(modes, 1600, 1200, 59), ModeInfo{})
	assert.Equal(t, getFirstModeBySizeRate(modes, 1280, 740, 60), ModeInfo{})

}

func Test_getRandrStatusStr(t *testing.T) {
	var status = []uint8{0, 1, 2, 3, 4}
	var statusstr = []string{"success", "invalid config time", "invalid time", "failed", "unknown status 4"}
	assert.Equal(t, getRandrStatusStr(status[0]), statusstr[0])
	assert.Equal(t, getRandrStatusStr(status[1]), statusstr[1])
	assert.Equal(t, getRandrStatusStr(status[2]), statusstr[2])
	assert.Equal(t, getRandrStatusStr(status[3]), statusstr[3])
	assert.Equal(t, getRandrStatusStr(status[4]), statusstr[4])
}
