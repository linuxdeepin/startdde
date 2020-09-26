package main

import (
	"bytes"
	"os/exec"
	"sort"
	"strings"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"golang.org/x/xerrors"
	"pkg.deepin.io/lib/log"
)

var logger *log.Logger
var _hasRandr1d2 bool // 是否 randr 版本大于等于 1.2

func init() {
	logger = log.NewLogger("deepin-greeter-display")
}

type Manager struct {
	xConn           *x.Conn
	configTimestamp x.Timestamp
	outputs         map[randr.Output]*Output
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

func main() {
	m, err := newManager()
	if err != nil {
		logger.Fatal("failed to new manager:", err)
	}

	if _hasRandr1d2 {
		m.listenEvent()
	} // else 直接退出
}
