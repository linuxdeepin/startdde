package display

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getMaxAreaSize(t *testing.T) {
	size := getMaxAreaSize([]Size{
		{1024, 768},
		{640, 480},
		{1280, 720},
		{800, 600},
	})
	assert.Equal(t, Size{1280, 720}, size)
	size = getMaxAreaSize(nil)
	assert.Equal(t, Size{}, size)
	size = getMaxAreaSize([]Size{
		{1024, 768},
	})
	assert.Equal(t, Size{1024, 768}, size)
}

func Test_filterModeInfos(t *testing.T) {
	modes := []ModeInfo{
		{
			Id:     1,
			name:   "1024x768",
			Width:  1024,
			Height: 768,
			Rate:   60.1,
		},
		{
			Id:     2,
			name:   "1024x768i",
			Width:  1024,
			Height: 768,
			Rate:   60.1,
		},
		{
			Id:     3,
			name:   "1024x768i",
			Width:  1024,
			Height: 768,
			Rate:   60.3,
		},
	}
	assert.Equal(t, []ModeInfo{
		{
			Id:     1,
			name:   "1024x768",
			Width:  1024,
			Height: 768,
			Rate:   60.1,
		},
	}, filterModeInfos(modes))

	// --------------------------
	modes = []ModeInfo{
		{
			Id:     1,
			name:   "1024x768",
			Width:  1024,
			Height: 768,
			Rate:   60.1,
		},
		{
			Id:     2,
			name:   "1024x768",
			Width:  1024,
			Height: 768,
			Rate:   60.10000001,
		},
		{
			Id:     3,
			name:   "1024x768",
			Width:  1024,
			Height: 768,
			Rate:   60.3,
		},
	}
	assert.Equal(t, []ModeInfo{
		{
			Id:     1,
			name:   "1024x768",
			Width:  1024,
			Height: 768,
			Rate:   60.1,
		},
		{
			Id:     3,
			name:   "1024x768",
			Width:  1024,
			Height: 768,
			Rate:   60.3,
		},
	}, filterModeInfos(modes))

	// --------------------------
	// 混合
	modes = []ModeInfo{
		{
			Id:     1,
			name:   "1024x768",
			Width:  1024,
			Height: 768,
			Rate:   60.1,
		},
		{
			Id:     2,
			name:   "1024x768",
			Width:  1024,
			Height: 768,
			Rate:   60.10000001,
		},
		{
			Id:     3,
			name:   "1024x768",
			Width:  1024,
			Height: 768,
			Rate:   60.3,
		},
		{
			Id:     4,
			name:   "1024x768i",
			Width:  1024,
			Height: 768,
			Rate:   60.1,
		},
		{
			Id:     5,
			name:   "1024x768i",
			Width:  1024,
			Height: 768,
			Rate:   60.3,
		},
	}
	assert.Equal(t, []ModeInfo{
		{
			Id:     1,
			name:   "1024x768",
			Width:  1024,
			Height: 768,
			Rate:   60.1,
		},
		{
			Id:     3,
			name:   "1024x768",
			Width:  1024,
			Height: 768,
			Rate:   60.3,
		},
	}, filterModeInfos(modes))
}
