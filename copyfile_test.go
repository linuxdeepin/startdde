/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
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

package main

import (
	"fmt"
	"os"
	"testing"
)

func testCopyFileInit() {
	os.Remove("/tmp/test")
	os.Remove("/tmp/test2")
	os.Remove("/tmp/ls")
	os.Symlink("/bin/ls", "/tmp/test")
}

func TestCopyFileNotKeepSymlink(t *testing.T) {
	testCopyFileInit()
	if err := copyFile("/bin/ls", "/tmp/ls", CopyFileNotKeepSymlink); err != nil {
		t.Fatal(err)
	} else {
		s, err := os.Stat("/tmp/ls")
		if (s.Mode() & os.ModeSymlink) != os.ModeSymlink {
			t.Log("pass copy a file with CopyFileNotKeepSymlink")
		} else {
			t.Error(err)
		}
	}

	if err := copyFile("/tmp/test", "/tmp/test2", CopyFileNotKeepSymlink); err != nil {
		t.Fatal(err)
	} else {
		s, err := os.Stat("/tmp/test2")
		if err != nil && s.Mode().IsRegular() {
			t.Fatal(err)
		} else {
			t.Log("pass copy a symlink with CopyFileNotKeepSymlink")
		}
	}
}
func TestCopyFileCopyFileNone(t *testing.T) {
	testCopyFileInit()
	os.Symlink("/bin/ls", "/tmp/ls")
	if err := copyFile("/tmp/test", "/tmp/ls", CopyFileNone); err != nil {
		t.Log("pass CopyFileKeepNone")
	} else {
		t.Error("overwrite existed file, copy file none failed")
	}
}

func TestCopyFileOverWrite(t *testing.T) {
	testCopyFileInit()
	if err := copyFile("/tmp/test", "/tmp/ls", CopyFileOverWrite); err != nil {
		t.Error("failed,", err)
	} else {
		s, err := os.Lstat("/tmp/ls")
		if err != nil {
			t.Error("failed,", err)
		} else {
			if (s.Mode() & os.ModeSymlink) == os.ModeSymlink {
				t.Log("pass CopyFileOverWrite")
			} else {
				t.Error("failed, dst is not symlink")
			}
		}
	}

	if err := copyFile("/tmp/test", "/tmp/ls",
		CopyFileNotKeepSymlink|CopyFileOverWrite); err != nil {
		fmt.Println(err)
	} else {
		s, err := os.Stat("/tmp/ls")
		if err != nil {
			t.Error("failed,", err)
		} else {
			if s.Mode().IsRegular() {
				t.Log("pass CopyFileNotKeepSymlink|CopyFileOverWrite")
			} else {
				t.Error("failed, dst is not a regular file")
			}
		}
	}
}
