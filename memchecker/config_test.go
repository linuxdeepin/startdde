// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package memchecker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_loadConfig(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		want    *configInfo
		wantErr bool
	}{
		{
			name: "loadConfig empty",
			args: args{
				filename: "./testdata/config-empty.json",
			},
			want:    &configInfo{},
			wantErr: false,
		},
		{
			name: "loadConfig wrong json",
			args: args{
				filename: "./testdata/config-wrong-json.json",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadConfig(tt.args.filename)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
