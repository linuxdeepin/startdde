package memanalyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	dbpath = "./testdata/memanalyzer.db"
)

func Test_config(t *testing.T) {
	assert.NotPanics(t, func() {
		_memDB, err := loadConfig(dbpath)
		assert.NotEqual(t, len(_memDB), 0)
		assert.NoError(t, err)

		setDB("/usr/share/applications/deepin-editor.desktop", 1024)
		value := getDB("/usr/share/applications/deepin-editor.desktop")
		assert.Equal(t, value, uint64(1024))

		setDB("/usr/share/applications/deepin-editor.desktop", 18668)
		err = doSaveDB(dbpath)
		assert.NoError(t, err)
	})
}
