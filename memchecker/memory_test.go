package memchecker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_doGetMemInfo(t *testing.T) {
	memInfo, err := doGetMemInfo("./testdata/meminfo")
	assert.Nil(t, err)
	assert.Equal(t, uint64(8073588), memInfo.MemTotal)
	assert.Equal(t, uint64(2278440), memInfo.MemFree)
	assert.Equal(t, uint64(5929504), memInfo.MemAvailable)
	assert.Equal(t, uint64(363508), memInfo.Buffers)
	assert.Equal(t, uint64(3254360), memInfo.Cached)
	assert.Equal(t, uint64(0), memInfo.SwapTotal)
	assert.Equal(t, uint64(0), memInfo.SwapFree)
	assert.Equal(t, uint64(0), memInfo.SwapCached)
}
