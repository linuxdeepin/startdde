package display

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestSensor(t *testing.T) {
	t.Run("Test Sensor", func(t *testing.T) {
		assert.NotPanics(t, initSensorListener)
		assert.NotPanics(t, startSensorListener)
		assert.NotPanics(t, stopSensorListener)
		assert.NotPanics(t, startSensorListener)
	})
}