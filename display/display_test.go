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
