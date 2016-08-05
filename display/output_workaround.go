package display

import (
	"time"
)

func fixEmptyOutput() {
	var ticker = time.NewTicker(time.Second * 5)
	for {
		select {
		case <-ticker.C:
			if anyOutputConnect() {
				return
			}
			tryEnableOutput()
		}
	}
}

func anyOutputConnect() bool {
	GetDisplayInfo().update()
	return (len(GetDisplayInfo().ListNames()) > 0)
}

func tryEnableOutput() {
	runCode("xrandr --auto")
	time.Sleep(time.Millisecond * 500)
}
