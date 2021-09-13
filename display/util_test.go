package display

import (
	"testing"

	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"github.com/stretchr/testify/assert"
)

func Test_getRotations(t *testing.T) {
	testdata := []struct {
		origin    uint16
		rotations []uint16
	}{
		{
			origin: randr.RotationRotate0,
			rotations: []uint16{
				randr.RotationRotate0,
			},
		},
		{
			origin: randr.RotationRotate90,
			rotations: []uint16{
				randr.RotationRotate90,
			},
		},
		{
			origin: randr.RotationRotate180,
			rotations: []uint16{
				randr.RotationRotate180,
			},
		},
		{
			origin: randr.RotationRotate270,
			rotations: []uint16{
				randr.RotationRotate270,
			},
		},
		{
			origin: randr.RotationRotate0 | randr.RotationRotate90,
			rotations: []uint16{
				randr.RotationRotate0,
				randr.RotationRotate90,
			},
		},
		{
			origin: randr.RotationRotate90 | randr.RotationRotate180,
			rotations: []uint16{
				randr.RotationRotate90,
				randr.RotationRotate180,
			},
		},
		{
			origin: 0xff,
			rotations: []uint16{
				randr.RotationRotate0,
				randr.RotationRotate90,
				randr.RotationRotate180,
				randr.RotationRotate270,
			},
		},
	}

	for _, v := range testdata {
		assert.ElementsMatch(t, getRotations(v.origin), v.rotations)
	}

}

func Test_getReflects(t *testing.T) {
	testdata := []struct {
		origin   uint16
		reflects []uint16
	}{
		{
			origin: randr.RotationReflectX,
			reflects: []uint16{
				0,
				randr.RotationReflectX,
			},
		},
		{
			origin: randr.RotationReflectY,
			reflects: []uint16{
				0,
				randr.RotationReflectY,
			},
		},
		{
			origin: 0xff,
			reflects: []uint16{
				0,
				randr.RotationReflectX,
				randr.RotationReflectY,
				randr.RotationReflectX | randr.RotationReflectY,
			},
		},
	}

	for _, v := range testdata {
		assert.ElementsMatch(t, getReflects(v.origin), v.reflects)
	}
}

func Test_parseCrtcRotation(t *testing.T) {
	testdata := []struct {
		origin   uint16
		rotation uint16
		reflect  uint16
	}{
		{
			origin:   randr.RotationRotate0 | randr.RotationReflectX,
			rotation: randr.RotationRotate0,
			reflect:  randr.RotationReflectX,
		},
		{
			origin:   randr.RotationRotate90 | randr.RotationReflectY,
			rotation: randr.RotationRotate90,
			reflect:  randr.RotationReflectY,
		},
		{
			origin:   randr.RotationRotate180 | randr.RotationReflectX | randr.RotationReflectY,
			rotation: randr.RotationRotate180,
			reflect:  randr.RotationReflectX | randr.RotationReflectY,
		},
		{
			origin:   randr.RotationRotate180 | randr.RotationRotate270 | randr.RotationReflectY,
			rotation: randr.RotationRotate0,
			reflect:  randr.RotationReflectY,
		},
	}

	for _, v := range testdata {
		rotation, reflect := parseCrtcRotation(v.origin)
		assert.Equal(t, rotation, v.rotation)
		assert.Equal(t, reflect, v.reflect)
	}
}

func Test_isDigit(t *testing.T) {
	testdata := []struct {
		data byte
		want bool
	}{
		{
			data: '0',
			want: true,
		},
		{
			data: '1',
			want: true,
		},
		{
			data: '2',
			want: true,
		},
		{
			data: '3',
			want: true,
		},
		{
			data: '4',
			want: true,
		},
		{
			data: '5',
			want: true,
		},
		{
			data: '6',
			want: true,
		},
		{
			data: '7',
			want: true,
		},
		{
			data: '8',
			want: true,
		},
		{
			data: '9',
			want: true,
		},
		{
			data: 'a',
			want: false,
		},
		{
			data: 'b',
			want: false,
		},
		{
			data: 'c',
			want: false,
		},
	}

	for _, v := range testdata {
		assert.Equal(t, isDigit(v.data), v.want)
	}
}

func Test_parseEDID(t *testing.T) {
	testdata := []struct {
		edid         []byte
		manufacturer string
		model        string
	}{
		{
			edid: []byte{
				0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x00, 0x5a, 0x63, 0x35, 0x83, 0x45, 0xa9, 0x00, 0x00,
				0x0f, 0x1e, 0x01, 0x03, 0x80, 0x35, 0x1e, 0x78, 0x2e, 0xdd, 0x75, 0xa5, 0x55, 0x4e, 0x9d, 0x27,
				0x0b, 0x50, 0x54, 0xbf, 0xef, 0x80, 0xb3, 0x00, 0xa9, 0x40, 0xa9, 0xc0, 0x95, 0x00, 0x90, 0x40,
				0x81, 0x80, 0x81, 0x40, 0x81, 0xc0, 0x02, 0x3a, 0x80, 0x18, 0x71, 0x38, 0x2d, 0x40, 0x58, 0x2c,
				0x45, 0x00, 0x0f, 0x28, 0x21, 0x00, 0x00, 0x1e, 0x00, 0x00, 0x00, 0xfd, 0x00, 0x32, 0x4b, 0x18,
				0x52, 0x12, 0x00, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x00, 0x00, 0x00, 0xfc, 0x00, 0x56,
				0x41, 0x32, 0x34, 0x37, 0x38, 0x2d, 0x48, 0x2d, 0x32, 0x0a, 0x20, 0x20, 0x00, 0x00, 0x00, 0xff,
				0x00, 0x56, 0x44, 0x57, 0x32, 0x30, 0x31, 0x35, 0x34, 0x33, 0x33, 0x33, 0x33, 0x0a, 0x01, 0x1f,
				0x02, 0x03, 0x2b, 0xf1, 0x58, 0x90, 0x05, 0x04, 0x03, 0x02, 0x07, 0x06, 0x08, 0x09, 0x0e, 0x0f,
				0x1f, 0x14, 0x13, 0x12, 0x11, 0x15, 0x16, 0x1d, 0x1e, 0x48, 0x49, 0x4a, 0x01, 0x23, 0x09, 0x7f,
				0x07, 0x83, 0x01, 0x00, 0x00, 0x65, 0x03, 0x0c, 0x00, 0x10, 0x00, 0x02, 0x3a, 0x80, 0x18, 0x71,
				0x38, 0x2d, 0x40, 0x58, 0x2c, 0x45, 0x00, 0x0f, 0x28, 0x21, 0x00, 0x00, 0x1e, 0x01, 0x1d, 0x80,
				0x18, 0x71, 0x1c, 0x16, 0x20, 0x58, 0x2c, 0x25, 0x00, 0x0f, 0x28, 0x21, 0x00, 0x00, 0x9e, 0x01,
				0x1d, 0x00, 0x72, 0x51, 0xd0, 0x1e, 0x20, 0x6e, 0x28, 0x55, 0x00, 0x0f, 0x28, 0x21, 0x00, 0x00,
				0x1e, 0x8c, 0x0a, 0xd0, 0x8a, 0x20, 0xe0, 0x2d, 0x10, 0x10, 0x3e, 0x96, 0x00, 0x0f, 0x28, 0x21,
				0x00, 0x00, 0x18, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x37,
			},
			manufacturer: "VSC",
			model:        "VA2478-H-2",
		},
	}

	for _, v := range testdata {
		manufacturer, model := parseEDID(v.edid)
		assert.Equal(t, manufacturer, v.manufacturer)
		assert.Equal(t, model, v.model)
	}
}

func Test_getOutputUUID(t *testing.T)  {
	testdata := struct {
		edid         []byte
	}{
		edid: []byte{
			0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x00, 0x5a, 0x63, 0x35, 0x83, 0x45, 0xa9, 0x00, 0x00,
			0x0f, 0x1e, 0x01, 0x03, 0x80, 0x35, 0x1e, 0x78, 0x2e, 0xdd, 0x75, 0xa5, 0x55, 0x4e, 0x9d, 0x27,
			0x0b, 0x50, 0x54, 0xbf, 0xef, 0x80, 0xb3, 0x00, 0xa9, 0x40, 0xa9, 0xc0, 0x95, 0x00, 0x90, 0x40,
			0x81, 0x80, 0x81, 0x40, 0x81, 0xc0, 0x02, 0x3a, 0x80, 0x18, 0x71, 0x38, 0x2d, 0x40, 0x58, 0x2c,
			0x45, 0x00, 0x0f, 0x28, 0x21, 0x00, 0x00, 0x1e, 0x00, 0x00, 0x00, 0xfd, 0x00, 0x32, 0x4b, 0x18,
			0x52, 0x12, 0x00, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x00, 0x00, 0x00, 0xfc, 0x00, 0x56,
			0x41, 0x32, 0x34, 0x37, 0x38, 0x2d, 0x48, 0x2d, 0x32, 0x0a, 0x20, 0x20, 0x00, 0x00, 0x00, 0xff,
			0x00, 0x56, 0x44, 0x57, 0x32, 0x30, 0x31, 0x35, 0x34, 0x33, 0x33, 0x33, 0x33, 0x0a, 0x01, 0x1f,
			0x02, 0x03, 0x2b, 0xf1, 0x58, 0x90, 0x05, 0x04, 0x03, 0x02, 0x07, 0x06, 0x08, 0x09, 0x0e, 0x0f,
			0x1f, 0x14, 0x13, 0x12, 0x11, 0x15, 0x16, 0x1d, 0x1e, 0x48, 0x49, 0x4a, 0x01, 0x23, 0x09, 0x7f,
			0x07, 0x83, 0x01, 0x00, 0x00, 0x65, 0x03, 0x0c, 0x00, 0x10, 0x00, 0x02, 0x3a, 0x80, 0x18, 0x71,
			0x38, 0x2d, 0x40, 0x58, 0x2c, 0x45, 0x00, 0x0f, 0x28, 0x21, 0x00, 0x00, 0x1e, 0x01, 0x1d, 0x80,
			0x18, 0x71, 0x1c, 0x16, 0x20, 0x58, 0x2c, 0x25, 0x00, 0x0f, 0x28, 0x21, 0x00, 0x00, 0x9e, 0x01,
			0x1d, 0x00, 0x72, 0x51, 0xd0, 0x1e, 0x20, 0x6e, 0x28, 0x55, 0x00, 0x0f, 0x28, 0x21, 0x00, 0x00,
			0x1e, 0x8c, 0x0a, 0xd0, 0x8a, 0x20, 0xe0, 0x2d, 0x10, 0x10, 0x3e, 0x96, 0x00, 0x0f, 0x28, 0x21,
			0x00, 0x00, 0x18, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x37,
		},
	}
	assert.Equal(t, getOutputUUID("HDMI-0",testdata.edid), "HDMI-0eaa56b1e94585d043c31a41870125b62")
}

func Test_toModeInfo(t *testing.T)  {
	testdate := randr.ModeInfo{
		Id:         84,
		Width:      1920,
		Height:     1080,
		DotClock:   148500000,
		HSyncStart: 2008,
		HSyncEnd:   2052,
		HTotal:     2200,
		HSkew:      0,
		VSyncStart: 1084,
		VSyncEnd:   1089,
		VTotal:     1125,
		Name:       "1920x1080",
		ModeFlags:  5,
	}

	assert.Equal(t, toModeInfo(testdate), ModeInfo{
		Id:     84,
		name:   "1920x1080",
		Width:  1920,
		Height: 1080,
		Rate:   60,
	})
}

func Test_outputSliceContains(t *testing.T) {
	testdate := []randr.Output{81,82}
	assert.Equal(t, outputSliceContains(testdate, randr.Output(81)), true)
	assert.Equal(t, outputSliceContains(testdate, randr.Output(83)), false)
}

func Test_getMonitorsCommonSizes(t *testing.T)  {
	var testdate = []*Monitor{
		{
			Modes: []ModeInfo{
				{
					Id:     84,
					Width:  1920,
					Height: 1080,
					Rate:   60.0,
				},
				{
					Id:     85,
					Width:  1920,
					Height: 1080,
					Rate:   50.0,
				},
				{
					Id:     90,
					Width:  1600,
					Height: 1200,
					Rate:   60,
				},
				{
					Id:     91,
					Width:  1680,
					Height: 1050,
					Rate:   60,
				},
				{
					Id:     92,
					Width:  1400,
					Height: 1050,
					Rate:   60,
				},
				{
					Id:     93,
					Width:  1600,
					Height: 900,
					Rate:   60,
				},
				{
					Id:     94,
					Width:  1280,
					Height: 1024,
					Rate:   60,
				},
				{
					Id:     96,
					Width:  1400,
					Height: 900,
					Rate:   60,
				},
				{
					Id:     97,
					Width:  1280,
					Height: 960,
					Rate:   60,
				},
				{
					Id:     98,
					Width:  1152,
					Height: 864,
					Rate:   75.0,
				},
				{
					Id:     99,
					Width:  1280,
					Height: 720,
					Rate:   60,
				},
			},
		},
		{
			Modes: []ModeInfo{
				{
					Id:     84,
					Width:  1920,
					Height: 1080,
					Rate:   60.0,
				},
				{
					Id:     85,
					Width:  1920,
					Height: 1080,
					Rate:   50.0,
				},
				{
					Id:     90,
					Width:  1600,
					Height: 1200,
					Rate:   60,
				},
				{
					Id:     91,
					Width:  1680,
					Height: 1050,
					Rate:   60,
				},
				{
					Id:     92,
					Width:  1400,
					Height: 1050,
					Rate:   60,
				},
				{
					Id:     93,
					Width:  1600,
					Height: 900,
					Rate:   60,
				},
				{
					Id:     94,
					Width:  1280,
					Height: 1024,
					Rate:   60,
				},
				{
					Id:     96,
					Width:  1400,
					Height: 900,
					Rate:   60,
				},
				{
					Id:     97,
					Width:  1280,
					Height: 960,
					Rate:   60,
				},
				{
					Id:     98,
					Width:  1152,
					Height: 864,
					Rate:   75.0,
				},
				{
					Id:     99,
					Width:  1280,
					Height: 720,
					Rate:   60,
				},
			},
		},
	}
	assert.Len(t, getMonitorsCommonSizes(testdate), 10)
	assert.NotNil(t, getMonitorsCommonSizes(testdate))
}

func Test_sortMonitorsByPrimaryAndID(t *testing.T){
	testdate := []*Monitor {
		{
			Name:"HDMI-0",
			ID:81,
		},
		{
			Name:"VGA-0",
			ID:82,
		},
	}

	sortMonitorsByPrimaryAndID(testdate, &Monitor{})
	assert.Equal(t, testdate, []*Monitor{
		{
			Name:"HDMI-0",
			ID:81,
		},
		{
			Name:"VGA-0",
			ID:82,
		},
	})

	sortMonitorsByPrimaryAndID(testdate, &Monitor{Name:"VGA-0"})
	assert.Equal(t, testdate, []*Monitor{
		{
			Name:"VGA-0",
			ID:82,
		},
		{
			Name:"HDMI-0",
			ID:81,
		},
	})
}

func Test_getMinIDMonitor(t *testing.T)  {
	testdate := []*Monitor {
		{
			Name:"HDMI-0",
			ID:81,
		},
		{
			Name:"VGA-0",
			ID:82,
		},
	}
	assert.Equal(t, getMinIDMonitor(testdate),&Monitor{Name:"HDMI-0", ID:81})
}

func Test_needSwapWidthHeight(t *testing.T) {
	assert.False(t, needSwapWidthHeight(1))
	assert.True(t, needSwapWidthHeight(2))
	assert.False(t, needSwapWidthHeight(4))
	assert.True(t, needSwapWidthHeight(8))
}
