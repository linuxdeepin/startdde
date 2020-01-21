package display

import (
	"pkg.deepin.io/dde/startdde/wl_display/org_kde_kwin/output_management"
	"pkg.deepin.io/dde/startdde/wl_display/org_kde_kwin/outputdevice"

	"github.com/dkolbly/wl"
)

func (m *Manager) loopDispatch() {
	for {
		select {
		case m.display.Context().Dispatch() <- struct{}{}:
		}
	}
}

type registryGlobalHandler struct {
	ch chan wl.RegistryGlobalEvent
}

func (rgh registryGlobalHandler) HandleRegistryGlobal(ev wl.RegistryGlobalEvent) {
	rgh.ch <- ev
}

type callbackDoneHandler struct {
	ch chan wl.CallbackDoneEvent
	cb *wl.Callback
}

func (cbh *callbackDoneHandler) Chan() <-chan wl.CallbackDoneEvent {
	return cbh.ch
}

func (cdh *callbackDoneHandler) Remove() {
	cdh.cb.RemoveDoneHandler(cdh)
}

func (cdh *callbackDoneHandler) HandleCallbackDone(ev wl.CallbackDoneEvent) {
	cdh.ch <- ev
}

type outputCfgAppliedHandler struct {
	ch chan output_management.OutputconfigurationAppliedEvent
}

func (h outputCfgAppliedHandler) HandleOutputconfigurationApplied(ev output_management.OutputconfigurationAppliedEvent) {
	h.ch <- ev
}

type outputCfgFailedHandler struct {
	ch chan output_management.OutputconfigurationFailedEvent
}

func (h outputCfgFailedHandler) HandleOutputconfigurationFailed(ev output_management.OutputconfigurationFailedEvent) {
	h.ch <- ev
}

func doSync(display *wl.Display) (*callbackDoneHandler, error) {
	callback, err := display.Sync()
	if err != nil {
		return nil, err
	}
	cbdChan := make(chan wl.CallbackDoneEvent)
	cbdHandler := &callbackDoneHandler{ch: cbdChan, cb: callback}
	callback.AddDoneHandler(cbdHandler)
	return cbdHandler, nil
}

func (m *Manager) registerGlobals() error {
	registry, err := m.display.GetRegistry()
	if err != nil {
		return err
	}

	rgeChan := make(chan wl.RegistryGlobalEvent)
	rgeHandler := registryGlobalHandler{ch: rgeChan}
	registry.AddGlobalHandler(rgeHandler)

	cbdHandler, err := doSync(m.display)
	if err != nil {
		return err
	}

loop:
	for {
		select {
		case ev := <-rgeChan:
			//logger.Debugf("ev: %#v\n", ev)
			err = m.registerInterface(registry, ev)
			if err != nil {
				logger.Warning(err)
			}
		case m.display.Context().Dispatch() <- struct{}{}:
		case <-cbdHandler.Chan():
			break loop
		}
	}

	// TODO: 可能不需要删除 global handler
	registry.RemoveGlobalHandler(rgeHandler)
	cbdHandler.Remove()
	return nil
}

func (m *Manager) registerInterface(registry *wl.Registry, ev wl.RegistryGlobalEvent) error {
	switch ev.Interface {

	case "org_kde_kwin_outputdevice":
		device := outputdevice.NewOutputdevice(m.display.Context())
		logger.Debug("register output device", device.Id())
		err := registry.Bind(ev.Name, ev.Interface, ev.Version, device)
		if err != nil {
			return err
		}
		odh := newOutputDeviceHandler(device)
		odh.doneCb = m.handleOutputDeviceDone
		odh.enabledCb = m.handleOutputDeviceEnabled

		m.devicesAllDoneMu.Lock()
		if !m.devicesAllDone {
			m.devicesWg.Add(1)
		}
		m.devicesAllDoneMu.Unlock()

		return nil

	case "org_kde_kwin_outputmanagement":
		outputManagement := output_management.NewOutputmanagement(m.display.Context())
		logger.Debug("register output management", outputManagement.Id())
		err := registry.Bind(ev.Name, ev.Interface, ev.Version, outputManagement)
		if err != nil {
			return err
		}

		m.management = outputManagement
	}
	return nil
}
