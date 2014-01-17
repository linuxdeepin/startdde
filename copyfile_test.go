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

func Test_CopyFile_NotKeepSymlink(t *testing.T) {
	testCopyFileInit()
	if err := CopyFile("/bin/ls", "/tmp/ls", CopyFileNotKeepSymlink); err != nil {
		t.Fatal(err)
	} else {
		s, err := os.Stat("/tmp/ls")
		if (s.Mode() & os.ModeSymlink) != os.ModeSymlink {
			t.Log("pass copy a file with CopyFileNotKeepSymlink")
		} else {
			t.Error(err)
		}
	}

	if err := CopyFile("/tmp/test", "/tmp/test2", CopyFileNotKeepSymlink); err != nil {
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
func Test_CopyFile_CopyFileNone(t *testing.T) {
	testCopyFileInit()
	os.Symlink("/bin/ls", "/tmp/ls")
	if err := CopyFile("/tmp/test", "/tmp/ls", CopyFileNone); err != nil {
		t.Log("pass CopyFileKeepNone")
	} else {
		t.Error("overwrite existed file, copy file none failed")
	}
}

func Test_CopyFile_OverWrite(t *testing.T) {
	testCopyFileInit()
	if err := CopyFile("/tmp/test", "/tmp/ls", CopyFileOverWrite); err != nil {
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

	if err := CopyFile("/tmp/test", "/tmp/ls",
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
