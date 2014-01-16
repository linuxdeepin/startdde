package main

import (
	"fmt"
	"os"
)

func testCopyNotKeepSymlink() {
	if err := CopyFile("/bin/ls", "/tmp/ls", CopyFileNotKeepSymlink); err != nil {
		fmt.Println(err)
	} else {
		s, err := os.Stat("/tmp/ls")
		if (s.Mode() & os.ModeSymlink) != os.ModeSymlink {
			fmt.Println("pass copy a file with CopyFileNotKeepSymlink")
		} else {
			fmt.Println(err)
		}
	}

	if err := CopyFile("/tmp/test", "/tmp/test2", CopyFileNotKeepSymlink); err != nil {
		fmt.Println(err)
	} else {
		s, err := os.Stat("/tmp/test2")
		if err != nil && s.Mode().IsRegular() {
			fmt.Println(err)
		} else {
			fmt.Println("pass copy a symlink with CopyFileNotKeepSymlink")
		}
	}
}
func testCopyFileNone() {
	if err := CopyFile("/tmp/test", "/tmp/ls", CopyFileNone); err != nil {
		fmt.Println("pass CopyFileKeepNone")
	} else {
		fmt.Println("overwrite existed file, copy file none failed")
	}
}

func testCopyFileOverWrite() {
	if err := CopyFile("/tmp/test", "/tmp/ls", CopyFileOverWrite); err != nil {
		fmt.Println("failed,", err)
	} else {
		s, err := os.Lstat("/tmp/ls")
		if err != nil {
			fmt.Println("failed,", err)
		} else {
			if (s.Mode() & os.ModeSymlink) == os.ModeSymlink {
				fmt.Println("pass CopyFileOverWrite")
			} else {
				fmt.Println("failed, dst is not symlink")
			}
		}
	}

	if err := CopyFile("/tmp/test", "/tmp/ls",
		CopyFileNotKeepSymlink|CopyFileOverWrite); err != nil {
		fmt.Println(err)
	} else {
		s, err := os.Stat("/tmp/ls")
		if err != nil {
			fmt.Println("failed,", err)
		} else {
			if s.Mode().IsRegular() {
				fmt.Println("pass CopyFileNotKeepSymlink|CopyFileOverWrite")
			} else {
				fmt.Println("failed, dst is not a regular file")
			}
		}
	}
}

func testCopyFileInit() {
	os.Remove("/tmp/test")
	os.Remove("/tmp/test2")
	os.Remove("/tmp/ls")
	os.Symlink("/bin/ls", "/tmp/test")
}

func testCopyFile() {
	testCopyFileInit()
	testCopyNotKeepSymlink()
	testCopyFileNone()
	testCopyFileOverWrite()
}
