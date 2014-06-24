package main

import (
	"pkg.linuxdeepin.com/lib/gio-2.0"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

type SubD struct {
	tryexec      bool
	isHidden     bool
	showInDeepin bool
}

type D struct {
	C string
	D string
	B bool
	S SubD
}

var d = []D{
	D{`
[Desktop Entry]
Type=Application
Exec=python
Hidden=false
Name=GoAgent
`,
		"Hidden=false",
		true,
		SubD{true, false, true}},
	D{`
[Desktop Entry]
Type=Application
Exec=python
Hidden=true
Name=GoAgent
`,
		"Hidden=true",
		false,
		SubD{true, true, true}},
	D{`
[Desktop Entry]
Type=Application
Exec=python
Hidden=false
OnlyShowIn=Deepin;kde
Name=GoAgent
`,
		"OnlyShowIn=Deepin;kde",
		true,
		SubD{true, false, true}},
	D{`
[Desktop Entry]
Type=Application
Exec=python
Hidden=false
OnlyShowIn=Unity
Name=GoAgent
`,
		"OnlyShowIn!=Deepin",
		false,
		SubD{true, false, false}},
	D{`
[Desktop Entry]
Type=Application
Exec=python
Hidden=false
NotShowIn=Unity
Name=GoAgent
`,
		"NotShowIn!=Deepin",
		true,
		SubD{true, false, true}},
	D{`
[Desktop Entry]
Type=Application
Exec=python
Hidden=false
NotShowIn=Deepin
Name=GoAgent
`,
		"NotShowIn==Deepin",
		false,
		SubD{true, false, false}},
	D{`
[Desktop Entry]
Type=Application
Exec=python
Hidden=false
TryExec=ls
Name=GoAgent
`,
		"TryExec=ls",
		true,
		SubD{true, false, true}},
	D{`
[Desktop Entry]
Type=Application
Exec=python
Hidden=false
TryExec=/bin/ls
Name=GoAgent
`,
		"TryExec==/bin/ls",
		true,
		SubD{true, false, true}},
	D{`
[Desktop Entry]
Type=Application
Exec=python
Hidden=false
TryExec=/bin/lssssss
Name=GoAgent
`,
		"TryExec==/bin/lssssss",
		false,
		SubD{false, false, true}},
	D{`
[Desktop Entry]
Type=Application
Exec=python
Hidden=false
TryExec=./lssssss
Name=GoAgent
`,
		"TryExec=./lssssss",
		false,
		SubD{false, false, true}},
}

func testAutostartRelatedFunc(pred func(string, D) bool) (string, bool) {
	for i := range d {
		p := fmt.Sprintf("/tmp/test%d.desktop", i)
		os.Remove(p)
	}
	for i, c := range d {
		p := fmt.Sprintf("/tmp/test%d.desktop", i)
		ioutil.WriteFile(p, []byte(c.C), os.ModePerm)
		if !pred(p, c) {
			return c.D, false
		}
		os.Remove(p)
	}
	return "", true
}

func TestHasValidTryExecKey(t *testing.T) {
	m := StartManager{}
	d, b := testAutostartRelatedFunc(func(name string, c D) bool {
		file := gio.NewDesktopAppInfoFromFilename(name)
		if file == nil {
			return true
		}
		defer file.Unref()
		return m.hasValidTryExecKey(file) == c.S.tryexec
	})
	if !b {
		t.Errorf("hasValidTryExecKey Failed: %s", d)
	}
}
func TestIsHidden(t *testing.T) {
	m := StartManager{}
	d, b := testAutostartRelatedFunc(func(name string, c D) bool {
		file := gio.NewDesktopAppInfoFromFilename(name)
		if file == nil {
			return true
		}
		defer file.Unref()
		return m.isHidden(file) == c.S.isHidden
	})
	if !b {
		t.Errorf("isHidden Failed: %s", d)
	}
}
func TestShowInDeepin(t *testing.T) {
	m := StartManager{}
	d, b := testAutostartRelatedFunc(func(name string, c D) bool {
		file := gio.NewDesktopAppInfoFromFilename(name)
		if file == nil {
			return true
		}
		defer file.Unref()
		return m.showInDeepin(file) == c.S.showInDeepin
	})
	if !b {
		t.Errorf("showInDeepin Failed: %s", d)
	}
}
func TestIsAutostartAux(t *testing.T) {
	m := StartManager{}
	d, b := testAutostartRelatedFunc(func(p string, c D) bool {
		return m.isAutostartAux(p) == c.B
	})
	if !b {
		t.Errorf("isAutostartAux Failed: %s", d)
	}
}

func TestIsAutostart(t *testing.T) {
	// m := StartManager{}
	// m.isAutostart()
}

func _TestSetAutostart(t *testing.T) {
	m := StartManager{}
	if err := m.setAutostart("dropbox.desktop", true); err != nil {
		fmt.Println(err)
	}
	if !m.isAutostart("dropbox.desktop") {
		t.Error("set to autostart failed")
	}
	if err := m.setAutostart("dropbox.desktop", false); err != nil {
		fmt.Println(err)
	}
	if m.isAutostart("dropbox.desktop") {
		t.Error("set to not autostart failed")
	}
}

func _TestScanDir(t *testing.T) {
	scanDir("/tmp", func(p string, info os.FileInfo) error {
		t.Log(info.Name())
		return nil
	})
}
