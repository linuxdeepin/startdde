/*
 * Copyright (C) 2016 ~ 2020 Deepin Technology Co., Ltd.
 *
 * Author:     hubenchang <hubenchang@uniontech.com>
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
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLaunchGroup(t *testing.T) {
	t.Run("Test launch groups", func(t *testing.T) {
		lg, err := doLoadGroupFile("./testdata/auto_launch/auto_launch.json")
		assert.NoError(t, err)
		assert.Equal(t, 2, lg.Len())
		assert.True(t, lg.Less(0, 1))
		lg.Swap(0, 1)
		assert.False(t, lg.Less(0, 1))
	})
}

func Test_doLoadGroupFile(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		want    launchGroups
		wantErr bool
	}{
		{
			name: "doLoadGroupFile",
			args: args{
				filename: "./testdata/auto_launch/auto_launch.json",
			},
			want: launchGroups{
				{
					Priority: 10,
					Group: []Cmd{
						{
							Command: "systemctl",
							Wait:    false,
							Args: []string{
								"--user",
								"restart",
								"deepin-turbo-booster-dtkwidget",
							},
						},
					},
				},
				{
					Priority: 7,
					Group: []Cmd{
						{
							Command: "/usr/lib/polkit-1-dde/dde-polkit-agent",
							Wait:    false,
							Args:    nil,
						},
						{
							Command: "dde-session-daemon-part2",
							Wait:    true,
							Args:    []string{},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "doLoadGroupFile not found",
			args: args{
				filename: "./testdata/auto_launch/auto_launch-notfound.json",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := doLoadGroupFile(tt.args.filename)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, tt.want, got)
		})
	}
}
