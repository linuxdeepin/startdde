package display

//func (m *Manager) loopDispatch() {
//	for {
//		select {
//		case m.display.Context().Dispatch() <- struct{}{}:
//		}
//	}
//}

//type registryGlobalHandler struct {
//	ch chan wl.RegistryGlobalEvent
//}
//
//func (rgh registryGlobalHandler) HandleRegistryGlobal(ev wl.RegistryGlobalEvent) {
//	rgh.ch <- ev
//}
//
//type callbackDoneHandler struct {
//	ch chan wl.CallbackDoneEvent
//	cb *wl.Callback
//}
//
//func (cbh *callbackDoneHandler) Chan() <-chan wl.CallbackDoneEvent {
//	return cbh.ch
//}
//
//func (cdh *callbackDoneHandler) Remove() {
//	cdh.cb.RemoveDoneHandler(cdh)
//}
//
//func (cdh *callbackDoneHandler) HandleCallbackDone(ev wl.CallbackDoneEvent) {
//	cdh.ch <- ev
//}
//
//type outputCfgAppliedHandler struct {
//	ch chan output_management.OutputconfigurationAppliedEvent
//}
//
//func (h outputCfgAppliedHandler) HandleOutputconfigurationApplied(ev output_management.OutputconfigurationAppliedEvent) {
//	h.ch <- ev
//}
//
//type outputCfgFailedHandler struct {
//	ch chan output_management.OutputconfigurationFailedEvent
//}
//
//func (h outputCfgFailedHandler) HandleOutputconfigurationFailed(ev output_management.OutputconfigurationFailedEvent) {
//	h.ch <- ev
//}
//
//func doSync(display *wl.Display) (*callbackDoneHandler, error) {
//	callback, err := display.Sync()
//	if err != nil {
//		return nil, err
//	}
//	cbdChan := make(chan wl.CallbackDoneEvent)
//	cbdHandler := &callbackDoneHandler{ch: cbdChan, cb: callback}
//	callback.AddDoneHandler(cbdHandler)
//	return cbdHandler, nil
//}
//
//func (m *Manager) registerGlobals() error {
//	registry, err := m.display.GetRegistry()
//	if err != nil {
//		return err
//	}
//	m.registry = registry
//
//	rgeChan := make(chan wl.RegistryGlobalEvent)
//	rgeHandler := registryGlobalHandler{ch: rgeChan}
//	registry.AddGlobalHandler(rgeHandler)
//
//	cbdHandler, err := doSync(m.display)
//	if err != nil {
//		return err
//	}
//
//loop:
//	for {
//		select {
//		case ev := <-rgeChan:
//			//logger.Debugf("ev: %#v\n", ev)
//			err = m.registerInterface(ev)
//			if err != nil {
//				logger.Warning(err)
//			}
//		case m.display.Context().Dispatch() <- struct{}{}:
//		case <-cbdHandler.Chan():
//			break loop
//		}
//	}
//
//	registry.RemoveGlobalHandler(rgeHandler)
//	cbdHandler.Remove()
//
//	registry.AddGlobalHandler(m)
//	registry.AddGlobalRemoveHandler(m)
//	return nil
//}

//func (m *Manager) HandleRegistryGlobal(ev wl.RegistryGlobalEvent) {
//	// handle monitor add
//	switch ev.Interface {
//	case "org_kde_kwin_outputdevice":
//		err := m.registerOutputDevice(ev)
//		if err != nil {
//			logger.Warningf("failed to register output device %v: %v", ev.Name, err)
//		}
//	}
//}
//
//func (m *Manager) HandleRegistryGlobalRemove(ev wl.RegistryGlobalRemoveEvent) {
//	// handle monitor remove
//	for id, monitor := range m.monitorMap {
//		if ev.Name == monitor.device.regName {
//			m.removeMonitor(id)
//			m.updatePropMonitors()
//			m.updateMonitorsId()
//			m.updateScreenSize()
//			return
//		}
//	}
//}

//func (m *Manager) registerOutputDevice(ev wl.RegistryGlobalEvent) error {
//	device := outputdevice.NewOutputdevice(m.display.Context())
//	logger.Debug("register output device", device.Id())
//	err := m.registry.Bind(ev.Name, ev.Interface, ev.Version, device)
//	if err != nil {
//		return err
//	}
//	odh := newOutputDeviceHandler(device, ev.Name)
//	odh.doneCb = m.handleOutputDeviceDone
//	odh.enabledCb = m.handleOutputDeviceEnabled
//	return nil
//}
//
//func (m *Manager) registerInterface(ev wl.RegistryGlobalEvent) error {
//	switch ev.Interface {
//
//	case "org_kde_kwin_outputdevice":
//		err := m.registerOutputDevice(ev)
//		if err != nil {
//			return err
//		}
//
//		m.devicesAllDoneMu.Lock()
//		if !m.devicesAllDone {
//			m.devicesWg.Add(1)
//		}
//		m.devicesAllDoneMu.Unlock()
//
//		return nil
//
//	case "org_kde_kwin_outputmanagement":
//		outputManagement := output_management.NewOutputmanagement(m.display.Context())
//		logger.Debug("register output management", outputManagement.Id())
//		err := m.registry.Bind(ev.Name, ev.Interface, ev.Version, outputManagement)
//		if err != nil {
//			return err
//		}
//
//		m.management = outputManagement
//	}
//	return nil
//}
