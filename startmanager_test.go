package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

type D struct {
	C string
	D string
	B bool
}

var d = []D{
	D{`
[Desktop Entry]
Type=Application
Exec=python /home/liliqiang/GAE/goagent/local/proxy.py
Hidden=false
Name=GoAgent
`,
		"Hidden=false",
		true},
	D{`
[Desktop Entry]
Type=Application
Exec=python /home/liliqiang/GAE/goagent/local/proxy.py
Hidden=true
Name=GoAgent
`,
		"Hidden=true",
		false},
	D{`
[Desktop Entry]
Type=Application
Exec=python /home/liliqiang/GAE/goagent/local/proxy.py
Hidden=false
OnlyShowIn=Deepin;kde
Name=GoAgent
`,
		"OnlyShowIn=Deepin;kde",
		true},
	D{`
[Desktop Entry]
Type=Application
Exec=python /home/liliqiang/GAE/goagent/local/proxy.py
Hidden=false
OnlyShowIn=Unity
Name=GoAgent
`,
		"OnlyShowIn!=Deepin",
		false},
	D{`
[Desktop Entry]
Type=Application
Exec=python /home/liliqiang/GAE/goagent/local/proxy.py
Hidden=false
NotShowIn=Unity
Name=GoAgent
`,
		"NotShowIn!=Deepin",
		true},
	D{`
[Desktop Entry]
Type=Application
Exec=python /home/liliqiang/GAE/goagent/local/proxy.py
Hidden=false
NotShowIn=Deepin
Name=GoAgent
`,
		"NotShowIn==Deepin",
		false},
	D{`
[Desktop Entry]
Type=Application
Exec=python /home/liliqiang/GAE/goagent/local/proxy.py
Hidden=false
TryExec=ls
Name=GoAgent
`,
		"Tryexec=ls",
		true},
	D{`
[Desktop Entry]
Type=Application
Exec=python /home/liliqiang/GAE/goagent/local/proxy.py
Hidden=false
TryExec=/bin/ls
Name=GoAgent
`,
		"TryExec==/bin/ls",
		true},
	D{`
[Desktop Entry]
Type=Application
Exec=python /home/liliqiang/GAE/goagent/local/proxy.py
Hidden=false
TryExec=/bin/lssssss
Name=GoAgent
`,
		"TryExec==/bin/lssssss",
		false},
	D{`
[Desktop Entry]
Type=Application
Exec=python /home/liliqiang/GAE/goagent/local/proxy.py
Hidden=false
TryExec=./lssssss
Name=GoAgent
`,
		"TryExec=./lssssss",
		false}}

func TestIsAutostartAux(t *testing.T) {
	m := StartManager{}
	for i, c := range d {
		p := fmt.Sprintf("/tmp/test%d.desktop", i)
		ioutil.WriteFile(p, []byte(c.C), os.ModePerm)
		if m.isAutostart(p) == c.B {
			t.Logf("Passed %s\n", c.D)
		} else {
			t.Errorf("Failed %s\n", c.D)
		}
		os.Remove(p)
	}
}
