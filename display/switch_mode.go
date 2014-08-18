package display

const (
	DisplayModeUnknow  = -100
	DisplayModeCustom  = 0
	DisplayModeMirrors = 1
	DisplayModeExtend  = 2
	DisplayModeOnlyOne = 3
)

func (dpy *Display) SwitchMode(mode int16, outputName string) {
	dpy.setPropDisplayMode(mode)

	switch mode {
	case DisplayModeMirrors:
		n := len(dpy.Monitors)
		if n <= 1 {
			return
		}
		for ; n > 1; n = len(dpy.Monitors) {
			dpy.JoinMonitor(dpy.Monitors[n-1].Name, dpy.Monitors[n-2].Name)
		}
		dpy.apply(false)
	case DisplayModeExtend:
		curX := int16(0)
		for _, m := range dpy.Monitors {
			dpy.SplitMonitor(m.Name)
		}
		for _, m := range dpy.Monitors {
			m.SwitchOn(true)
			m.SetPos(curX, 0)
			m.SetMode(m.BestMode.ID)
			curX += int16(m.BestMode.Width)
		}
		dpy.apply(true)
	case DisplayModeOnlyOne:
		func() {
			dpy.lockMonitors()
			outputNameValid := GetDisplayInfo().QueryOutputs(outputName) != 0
			//validValue := mode >= DisplayModeOnlyOne && int(mode) <= len(dpy.Monitors)
			dpy.unlockMonitors()

			if outputNameValid {
				for _, m := range dpy.Monitors {
					dpy.SplitMonitor(m.Name)
				}

				for _, m := range dpy.Monitors {
					if m.Name == outputName {
						m.SetPos(0, 0)
						m.SetMode(m.BestMode.ID)
						//m.changeSwitchOn(true)
						m.SwitchOn(true)
						dpy.changePrimary(m.Name)
					}
				}
				for _, m := range dpy.Monitors {
					if m.Name != outputName {
						//m.changeSwitchOn(false)
						m.SwitchOn(false)
					}
				}
				dpy.apply(false)
			}
		}()
	case DisplayModeCustom:
		dpy.ResetChanges()
	}
}
