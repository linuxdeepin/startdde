package main

import (
	"bytes"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/log"
)

const (
	sensorProxyServiceName     = "com.deepin.SensorProxy"
	sensorProxyPath            = "/com/deepin/SensorProxy"
	sensorProxySignalName      = "RotationValueChanged"
	sensorProxySignal          = "com.deepin.SensorProxy.RotationValueChanged"
	sensorProxyGetScreenStatus = "com.deepin.SensorProxy.GetScreenStatus"
)

var (
	logger              *log.Logger
	rotationScreenValue = map[string]string{
		"normal": "normal",
		"left":   "right", // 屏幕重力旋转左转90
		"right":  "left",  // 屏幕重力旋转右转90
	}
)

func init() {
	logger = log.NewLogger("deepin-greeter-rotate")
}

type Manager struct {
	sysBus                          *dbus.Conn
	service                         *dbusutil.Service
	rotateScreenTimeDelay           int32
	startBuildInScreenRotationMutex sync.Mutex
}

func newManager(service *dbusutil.Service) *Manager {
	var err error
	var defaultRotateScreenTimeDelay int32 = 500

	m := &Manager{
		service: service,
	}

	// 从配置文件中读取延迟屏幕旋转的时间，没有配置使用默认时间500ms
	content, err := ioutil.ReadFile(filepath.Join("/var/lib/deepin/greeter-rotation-time",
		"time-config"))
	if err != nil {
		m.rotateScreenTimeDelay = defaultRotateScreenTimeDelay
		logger.Warningf("fail to read config file: %v, use default delay time: %v", err, m.rotateScreenTimeDelay)
	} else {
		time, err := strconv.Atoi(string(bytes.TrimSpace(content)))
		if err != nil {
			m.rotateScreenTimeDelay = defaultRotateScreenTimeDelay
			logger.Warningf("fail to parse time parameter: %v, use default delay time: %v", err, m.rotateScreenTimeDelay)
		}

		logger.Debug("rotate delay time:", time)
		m.rotateScreenTimeDelay = int32(time)
	}

	m.sysBus, err = dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
	}

	m.initScreenRotation()

	return m
}

/* 根据从内核获取的显示屏幕的初始状态(屏幕旋转的方向)，旋转登录界面到对应的方向 */
func (m *Manager) initScreenRotation() {
	screenRatationStatus := "normal"
	screenRatationStatus, err := m.getScreenRatationStatus()
	logger.Debug("init screen rotation status:", screenRatationStatus)
	if err != nil {
		logger.Warning("failed to get screen rotation status")
		return
	}

	m.startBuildInScreenRotationMutex.Lock()
	defer m.startBuildInScreenRotationMutex.Unlock()
	rotationRotate, ok := rotationScreenValue[strings.TrimSpace(screenRatationStatus)]
	if ok {
		m.startScreenRotation(rotationRotate)
	}
}

/* 监听内核屏幕旋转的信号，旋转登录界面显示到对应方向 */
func (m *Manager) listenRotateSignal() {
	err := m.sysBus.BusObject().AddMatchSignal(sensorProxyServiceName, sensorProxySignalName,
		dbus.WithMatchObjectPath(dbus.ObjectPath(sensorProxyPath)), dbus.WithMatchSender(sensorProxyServiceName)).Err
	if err != nil {
		logger.Fatal(err)
	}

	signalCh := make(chan *dbus.Signal, 10)
	m.sysBus.Signal(signalCh)

	go func() {
		var rotationScreenTimer *time.Timer
		rotateScreenValue := "normal"

		for sig := range signalCh {
			if sig.Path != sensorProxyPath || sig.Name != sensorProxySignal {
				continue
			}

			err := dbus.Store(sig.Body, &rotateScreenValue)
			if err != nil {
				logger.Warning("call dbus.Store err:", err)
				continue
			}

			if rotationScreenTimer == nil {
				rotationScreenTimer = time.AfterFunc(time.Millisecond*time.Duration(m.rotateScreenTimeDelay), func() {
					m.startBuildInScreenRotationMutex.Lock()
					defer m.startBuildInScreenRotationMutex.Unlock()
					rotationRotate, ok := rotationScreenValue[strings.TrimSpace(rotateScreenValue)]
					if ok {
						m.startScreenRotation(rotationRotate)
					}
				})
			} else {
				rotationScreenTimer.Reset(time.Millisecond * time.Duration(m.rotateScreenTimeDelay))
			}
		}
	}()
}

/* 收到内核旋转屏幕的信号后，调用xrandr命令将登录界面旋转到对应方向 */
func (m *Manager) startScreenRotation(currentRotateScreenValue string) {
	// 此处是针对长城一体机(屏幕类型为LVDS)的定制，所以先写死
	// 后期如果需要支持其它显示屏，可放开
	err := exec.Command("xrandr", "--output", "LVDS", "--rotate", currentRotateScreenValue).Run()
	if err != nil {
		logger.Warning("failed to rotate screen by xrandr command:", err)
	}
}

func (m *Manager) getSensorProxyDBus() (dbus.BusObject, error) {
	var sensorProxy dbus.BusObject
	if sensorProxy != nil {
		return sensorProxy, nil
	}

	sensorProxy = m.sysBus.Object(sensorProxyServiceName, sensorProxyPath)
	return sensorProxy, nil
}

func (m *Manager) getScreenRatationStatus() (string, error) {
	obj, err := m.getSensorProxyDBus()
	if err != nil {
		return "", err
	}

	var status string
	err = obj.Call(sensorProxyGetScreenStatus, 0).Store(&status)
	if err != nil {
		return "", err
	}

	return status, nil
}

func main() {
	service, err := dbusutil.NewSystemService()
	if err != nil {
		logger.Fatal("failed to new system service:", err)
	}

	m := newManager(service)
	if err != nil {
		logger.Fatal("failed to new manager:", err)
	}

	m.listenRotateSignal()

	service.Wait()
}
