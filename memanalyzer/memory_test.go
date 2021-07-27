package memanalyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_sumMemByFile(t *testing.T) {
	sum, err := sumMemByFile("./testdata/proc_pid_status")
	assert.NoError(t, err)
	assert.Equal(t, uint64(0x7080), sum)
}
