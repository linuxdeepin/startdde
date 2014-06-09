package main

import "fmt"
import "io"
import "crypto/rand"
import "os"
import "os/exec"
import "time"

func genUuid() string {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(rand.Reader, uuid)
	if n != len(uuid) || err != nil {
		panic("This can failed?")
	}
	// variant bits; see section 4.1.1
	uuid[8] = uuid[8]&^0xc0 | 0x80
	// version 4 (pseudo-random); see section 4.1.3
	uuid[6] = uuid[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}

func (m *SessionManager) launch(bin string, wait bool) bool {
	id := genUuid()
	cmd := exec.Command(bin)

	if !wait {
		cmd.Start()
		return true
	}

	cmd.Env = append(os.Environ(), fmt.Sprintf("DDE_SESSION_PROCESS_COOKIE_ID=%s", id))
	m.cookies[id] = make(chan time.Time, 1)
	startStamp := time.Now()
	cmd.Start()

	select {
	case endStamp := <-m.cookies[id]:
		delete(m.cookies, id)
		Logger.Info(bin, "StartDuration:", endStamp.Sub(startStamp))
		return true
	case endStamp := <-time.After(time.Second * 10):
		Logger.Info(bin, "timeout:", endStamp.Sub(startStamp))
		return false
	}
}
func (m *SessionManager) Register(id string) bool {
	if cookie, ok := m.cookies[id]; ok {
		cookie <- time.Now()
		return true
	}
	return false
}
