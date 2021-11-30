package main

import (
	"bytes"
	"github.com/linuxdeepin/go-x11-client/ext/input"
	"github.com/linuxdeepin/go-x11-client/ext/xfixes"
	"os/exec"
	"sort"
	"strings"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"golang.org/x/xerrors"
	"github.com/linuxdeepin/go-lib/log"
)

var logger *log.Logger
var _hasRandr1d2 bool // 是否 randr 版本大于等于 1.2

const evMaskForHideCursor uint32 = input.XIEventMaskRawMotion | input.XIEventMaskRawTouchBegin

func init() {
	logger = log.NewLogger("deepin-greeter-display")
}

type Manager struct {
	xConn           *x.Conn
	configTimestamp x.Timestamp
	outputs         map[randr.Output]*Output
	cursorShowed        bool
}

type Output struct {
	id        randr.Output
	name      string
	connected bool
}

func newManager() (*Manager, error) {
	m := &Manager{
		outputs: make(map[randr.Output]*Output),
	}
	var err error
	m.xConn, err = x.NewConn()
	if err != nil {
		return nil, xerrors.Errorf("failed to connect X: %w", err)
	}

	randrVersion, err := randr.QueryVersion(m.xConn, randr.MajorVersion, randr.MinorVersion).Reply(m.xConn)
	if err != nil {
		return nil, err
	}
	if randrVersion.ServerMajorVersion > 1 ||
		(randrVersion.ServerMajorVersion == 1 && randrVersion.ServerMinorVersion >= 2) {
		_hasRandr1d2 = true
	}

	if _hasRandr1d2 {
		resources, err := m.getScreenResources()
		if err != nil {
			return nil, xerrors.Errorf("failed to get screen resources: %w", err)
		}

		m.configTimestamp = resources.ConfigTimestamp

		for _, output := range resources.Outputs {
			outputInfo, err := m.getOutputInfo(output)
			if err != nil {
				return nil, xerrors.Errorf("failed to get output %d info: %w", output, err)
			}

			m.outputs[output] = &Output{
				id:        output,
				name:      outputInfo.Name,
				connected: outputInfo.Connection == randr.ConnectionConnected,
			}
		}
	}
	m.cursorShowed = true
	m.initXExtensions()

	return m, nil
}

func (m *Manager) listenEvent() {
	eventChan := m.xConn.MakeAndAddEventChan(50)
	root := m.xConn.GetDefaultScreen().Root
	err := randr.SelectInputChecked(m.xConn, root,
		randr.NotifyMaskOutputChange|randr.NotifyMaskScreenChange).Check(m.xConn)
	if err != nil {
		logger.Warning("failed to select randr event:", err)
		return
	}

	rrExtData := m.xConn.GetExtensionData(randr.Ext())
	inputExtData := m.xConn.GetExtensionData(input.Ext())

	for ev := range eventChan {
		switch ev.GetEventCode() {
		case randr.NotifyEventCode + rrExtData.FirstEvent:
			event, _ := randr.NewNotifyEvent(ev)
			switch event.SubCode {
			case randr.NotifyOutputChange:
				e, _ := event.NewOutputChangeNotifyEvent()
				m.handleOutputChanged(e)
			}

		case randr.ScreenChangeNotifyEventCode + rrExtData.FirstEvent:
			event, _ := randr.NewScreenChangeNotifyEvent(ev)
			m.handleScreenChanged(event)

		case x.GeGenericEventCode:
			geEvent, _ := x.NewGeGenericEvent(ev)
			if geEvent.Extension == inputExtData.MajorOpcode {
				switch geEvent.EventType {
				case input.RawMotionEventCode:
					m.beginMoveMouse()
					_, err := m.queryPointer()
					if err != nil {
						logger.Warning(err)
					}

				case input.RawTouchBeginEventCode:
					m.beginTouch()
					_, err := m.queryPointer()
					if err != nil {
						logger.Warning(err)
					}
				}
			}
		}
	}
}

func (m *Manager) getScreenResources() (*randr.GetScreenResourcesReply, error) {
	root := m.xConn.GetDefaultScreen().Root
	resources, err := randr.GetScreenResources(m.xConn, root).Reply(m.xConn)
	return resources, err
}

func (m *Manager) getOutputInfo(output randr.Output) (*randr.GetOutputInfoReply, error) {
	cfgTs := m.configTimestamp

	outputInfo, err := randr.GetOutputInfo(m.xConn, output, cfgTs).Reply(m.xConn)
	if err != nil {
		return nil, err
	}
	if outputInfo.Status != randr.StatusSuccess {
		return nil, xerrors.Errorf("status is not success, is %v", outputInfo.Status)
	}
	return outputInfo, err
}

func (m *Manager) handleOutputChanged(ev *randr.OutputChangeNotifyEvent) {
	logger.Debug("output changed", ev.Output)

	outputInfo, err := m.getOutputInfo(ev.Output)
	if err != nil {
		logger.Warningf("failed to get output %d info: %v", ev.Output, err)
		return
	}

	oldConnected := false
	o, ok := m.outputs[ev.Output]
	if ok {
		oldConnected = o.connected
	}

	connected := outputInfo.Connection == randr.ConnectionConnected
	m.outputs[ev.Output] = &Output{
		id:        ev.Output,
		name:      outputInfo.Name,
		connected: connected,
	}

	if oldConnected != connected {
		m.configure()
	}
}

func (m *Manager) handleScreenChanged(ev *randr.ScreenChangeNotifyEvent) {
	logger.Debugf("screen changed cfgTs: %v", ev.ConfigTimestamp)
	if m.configTimestamp != ev.ConfigTimestamp {
		m.configTimestamp = ev.ConfigTimestamp
	}
}

func (m *Manager) configure() {
	var connectedOutputs []*Output
	for _, output := range m.outputs {
		if output.connected {
			connectedOutputs = append(connectedOutputs, output)
		}
	}
	sort.Slice(connectedOutputs, func(i, j int) bool {
		return connectedOutputs[i].id < connectedOutputs[j].id
	})

	var args []string
	var prev string
	first := true
	for _, output := range connectedOutputs {
		args = append(args, "--output", output.name, "--auto")

		if first {
			args = append(args, "--pos", "0x0")
		}
		if prev != "" {
			args = append(args, "--right-of", prev)
		}

		first = false
		prev = output.name
	}

	for _, output := range m.outputs {
		if !output.connected {
			args = append(args, "--output", output.name, "--off")
		}
	}

	logger.Debugf("$ xrandr %s", strings.Join(args, " "))
	cmd := exec.Command("xrandr", args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		logger.Warningf("xrandr exit err: %v, stderr: %s", err, errBuf.Bytes())
	}
}

func (m *Manager) initXExtensions() {
	_, err := xfixes.QueryVersion(m.xConn, xfixes.MajorVersion, xfixes.MinorVersion).Reply(m.xConn)
	if err != nil {
		logger.Warning(err)
	}

	_, err = input.XIQueryVersion(m.xConn, input.MajorVersion, input.MinorVersion).Reply(m.xConn)
	if err != nil {
		logger.Warning(err)
		return
	}


	err = m.doXISelectEvents(evMaskForHideCursor)
	if err != nil {
		logger.Warning(err)
	}

}

func (m *Manager) doXISelectEvents(evMask uint32) error {
	root := m.xConn.GetDefaultScreen().Root
	err := input.XISelectEventsChecked(m.xConn, root, []input.EventMask{
		{
			DeviceId: input.DeviceAllMaster,
			Mask:     []uint32{evMask},
		},
	}).Check(m.xConn)
	return err
}

func (m *Manager) queryPointer() (*x.QueryPointerReply, error) {
	root := m.xConn.GetDefaultScreen().Root
	reply, err := x.QueryPointer(m.xConn, root).Reply(m.xConn)
	return reply, err
}

func (m *Manager) beginMoveMouse() {
	if m.cursorShowed {
		return
	}
	err := m.doShowCursor(true)
	if err != nil {
		logger.Warning(err)
	}
	m.cursorShowed = true
}

func (m *Manager) beginTouch() {
	if !m.cursorShowed {
		return
	}
	err := m.doShowCursor(false)
	if err != nil {
		logger.Warning(err)
	}
	m.cursorShowed = false
}
func (m *Manager) doShowCursor(show bool) error {
	rootWin := m.xConn.GetDefaultScreen().Root
	var cookie x.VoidCookie
	if show {
		logger.Debug("xfixes show cursor")
		cookie = xfixes.ShowCursorChecked(m.xConn, rootWin)
	} else {
		logger.Debug("xfixes hide cursor")
		cookie = xfixes.HideCursorChecked(m.xConn, rootWin)
	}
	err := cookie.Check(m.xConn)
	return err
}

func main() {
	m, err := newManager()
	if err != nil {
		logger.Fatal("failed to new manager:", err)
	}

	if _hasRandr1d2 {
		m.listenEvent()
	} // else 直接退出
}
