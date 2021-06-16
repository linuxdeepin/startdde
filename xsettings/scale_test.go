/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
				assert.NotNil(t, err)
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
