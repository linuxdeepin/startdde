// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package wm_kwin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getWMSwitchLastWm(t *testing.T) {
	tests := []struct {
		name       string
		configHome string
		wantLastWm string
		wantErr    bool
	}{
		{
			name:       "getWMSwitchLastWm",
			configHome: "testdata/lastwm",
			wantLastWm: "deepin-wm",
			wantErr:    false,
		},
		{
			name:       "getWMSwitchLastWm not found",
			configHome: "testdata/lastwm-",
			wantLastWm: "",
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			abs, _ := filepath.Abs(tt.configHome)
			os.Setenv("XDG_CONFIG_HOME", abs)
			defer os.Unsetenv("XDG_CONFIG_HOME")

			gotLastWm, err := getWMSwitchLastWm()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, tt.wantLastWm, gotLastWm)
		})
	}
}
