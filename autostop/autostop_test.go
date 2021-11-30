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

package autostop

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/linuxdeepin/go-lib/log"
)

func Test_doScanScripts(t *testing.T) {
	type args struct {
		dir string
	}
	cases := []struct {
		args    args
		want    []string
		wantErr bool
	}{
		{
			args: args{
				dir: "testdata/scripts",
			},
			want: []string{
				"testdata/scripts/hello.sh",
				"testdata/scripts/ls.sh",
			},
			wantErr: false,
		},
	}
	for _, _case := range cases {
		got, err := doScanScripts(_case.args.dir)
		if _case.wantErr {
			assert.Error(t, err)
		}

		assert.ElementsMatch(t, _case.want, got)
	}
}

func Test_launchScripts(t *testing.T) {
	logger = log.NewLogger("test/autostop")
	type args struct {
		scripts []string
	}
	cases := []struct {
		args args
		errs []error
	}{
		{
			args: args{
				scripts: []string{
					"testdata/scripts/hello.sh",
					"testdata/scripts/ls.sh",
				},
			},
			errs: nil,
		},
	}
	for _, _case := range cases {
		errs := launchScripts(_case.args.scripts)
		for _, err := range errs {
			println(err.Error())
		}
		assert.ElementsMatch(t, _case.errs, errs)
	}
}

func Test_getScripts(t *testing.T) {
	cases := []struct {
		dirs  []string
		wants []string
	}{
		{
			dirs: []string{
				"testdata/path-not-exist",
			},
			wants: []string{},
		},
		{
			dirs: []string{
				"testdata/scripts",
			},
			wants: []string{
				"testdata/scripts/ls.sh",
				"testdata/scripts/hello.sh",
			},
		},
	}
	for _, _case := range cases {
		got := getScripts(_case.dirs)
		assert.ElementsMatch(t, _case.wants, got)
	}
}
