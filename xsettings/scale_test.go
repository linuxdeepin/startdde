// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package xsettings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getPlymouthTheme(t *testing.T) {
	type args struct {
		file string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "getPlymouthTheme",
			args: args{
				file: "./testdata/plymouth-theme.ini",
			},
			want:    "testxxx",
			wantErr: false,
		},
		{
			name: "getPlymouthTheme not found",
			args: args{
				file: "./testdata/plymouth-theme-not-found.ini",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getPlymouthTheme(tt.args.file)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_getPlymouthThemeScaleFactor(t *testing.T) {
	type args struct {
		theme string
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "getPlymouthThemeScaleFactor",
			args: args{
				theme: "deepin-logo",
			},
			want: 1,
		},
		{
			name: "getPlymouthThemeScaleFactor hidpi",
			args: args{
				theme: "deepin-hidpi-logo",
			},
			want: 2,
		},
		{
			name: "getPlymouthThemeScaleFactor not found",
			args: args{
				theme: "depin-hidpi-logo",
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPlymouthThemeScaleFactor(tt.args.theme)
			assert.Equal(t, tt.want, got)
		})
	}
}
