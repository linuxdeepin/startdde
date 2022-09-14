// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"testing"
)

func testCopyFileInit() {
	err := os.Remove("/tmp/test")
	if err != nil {
		logger.Warning(err)
	}
	err = os.Remove("/tmp/test2")
	if err != nil {
		logger.Warning(err)
	}
	err = os.Remove("/tmp/ls")
	if err != nil {
		logger.Warning(err)
	}
	err = os.Symlink("/bin/ls", "/tmp/test")
	if err != nil {
		logger.Warning(err)
	}
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
	err := os.Symlink("/bin/ls", "/tmp/ls")
	if err != nil {
		logger.Warning(err)
	}
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
