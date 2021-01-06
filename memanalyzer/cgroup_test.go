package memanalyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_strvToUint16(t *testing.T) {
	testdata := struct {
		from []string
		to   []uint16
	}{
		from: []string{
			"12",
			"99",
			"ac",
		},

		to: []uint16{
			uint16(12),
			uint16(99),
			uint16(0),
		},
	}

	value := strvToUint16(testdata.from)
	assert.ElementsMatch(t, testdata.to, value)
}
