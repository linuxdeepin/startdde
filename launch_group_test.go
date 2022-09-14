// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

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
