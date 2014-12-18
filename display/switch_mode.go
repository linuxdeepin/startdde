package display

const (
	DisplayModeUnknow  = -100
	DisplayModeCustom  = 0
	DisplayModeMirrors = 1
	DisplayModeExtend  = 2
	DisplayModeOnlyOne = 3
)

func (dpy *Display) SwitchMode(mode int16, outputName string) {

	switch mode {
	case DisplayModeMirrors:
		n := len(dpy.Monitors)
		if n == 0 {
			logger.Error("Invoking SwitchMode with none Monitors.")
			return
		}
		if n == 1 {
			m := dpy.Monitors[0]
			m.SetPos(0, 0)
			m.SetMode(m.BestMode.ID)
			m.SwitchOn(true)
		} else {
			for ; n > 1; n = len(dpy.Monitors) {
				dpy.JoinMonitor(dpy.Monitors[n-1].Name, dpy.Monitors[n-2].Name)
			}
		}
		dpy.apply(false)

		dpy.setPropDisplayMode(mode)
		dpy.saveDisplayMode(mode, "")
	case DisplayModeExtend:
		for _, m := range dpy.Monitors {
			dpy.SplitMonitor(m.Name)
		}

		curX := int16(0)
		for _, m := range dpy.Monitors {
			m.changeSwitchOn(true)
			m.cfg.Enabled = true
			m.SetPos(curX, 0)
			m.SetMode(m.BestMode.ID)
			curX += int16(m.BestMode.Width)
		}
		dpy.apply(false)

		dpy.setPropDisplayMode(mode)
		dpy.saveDisplayMode(mode, "")
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
						m.SwitchOn(true)
						dpy.changePrimary(m.Name, true)
					}
				}
				for _, m := range dpy.Monitors {
					if m.Name != outputName {
						m.SwitchOn(false)
					}
				}
				dpy.apply(false)

				dpy.setPropDisplayMode(mode)
				dpy.saveDisplayMode(mode, outputName)
			}
		}()
	case DisplayModeCustom:
		dpy.setPropDisplayMode(mode)
		dpy.saveDisplayMode(mode, "")
		dpy.ResetChanges()
	}
	dpy.detectChanged()
}
