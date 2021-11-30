package iowait

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/linuxdeepin/go-lib/log"
)

func Test_showIOWait(t *testing.T) {
	_logger = log.NewLogger("startdde")
	assert.NotPanics(t, func() {
		showIOWait()
	})
}
