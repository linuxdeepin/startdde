// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

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
