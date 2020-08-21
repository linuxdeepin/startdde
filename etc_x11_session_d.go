package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"

	x "github.com/linuxdeepin/go-x11-client"
	"pkg.deepin.io/dde/api/userenv"
	"pkg.deepin.io/lib/xdg/basedir"
)

// 00deepin-dde-env
func runScript00DeepinDdeEnvFaster() {
	envMap, err := userenv.Load()
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warning("user env load failed:", err)
		}
		return
	}
	for key, value := range envMap {
		_envVars[key] = value
	}
}

// 01deepin-profile
func runScript01DeepinProfileFaster() {
	// NOTE: 放在启动核心组件之后
	_homeDir = basedir.GetUserHomeDir()

	// /etc/profile/deepin-xdg-dir.sh
	_envVars["XDG_DATA_HOME"] = filepath.Join(_homeDir, ".local/share")
	_envVars["XDG_CONFIG_HOME"] = filepath.Join(_homeDir, ".config")
	_envVars["XDG_CACHE_HOME"] = filepath.Join(_homeDir, ".cache")
}

// 05uos-profile
func runScript05UosProfileFaster() {
	// loongson config
	_, err := os.Stat("/dev/galcore")
	if err != nil {
		return
	}
	_envVars["KWIN_COMPOSE"] = "N"
	_envVars["DESKTOP_CAN_SCREENSAVER"] = "N"
	_envVars["HDMI_CAN_BRIGHTNESS"] = "N"
}

// 20dbus_xdg-runtime
func runScript20DBusXdgRuntimeFaster() {
	envSessionBusAddr := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	envXdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
	uid := os.Getuid()

	if envSessionBusAddr == "" && envXdgRuntimeDir != "" &&
		envXdgRuntimeDir == "/run/user/"+strconv.Itoa(uid) {
		_, err := os.Stat(filepath.Join(envXdgRuntimeDir, "bus"))
		if err == nil {
			_envVars["DBUS_SESSION_BUS_ADDRESS"] = "unix:path=" + envXdgRuntimeDir + "/bus"
		}
	}

	for _, varName := range []string{"DISPLAY", "XAUTHORITY", "WAYLAND_DISPLAY"} {
		val, ok := os.LookupEnv(varName)
		if ok {
			_envVars[varName] = val
		}
	}
}

// 30x11-common-xresources
func runScript30X11CommonXResourcesFaster() {
	// NOTE: 放在启动核心组件之后

	xrdbBin, err := exec.LookPath("xrdb")
	if err != nil {
		return
	}

	const sysResourcesDir = "/etc/X11/Xresources"
	fileInfos, err := ioutil.ReadDir(sysResourcesDir)
	if err != nil {
		logger.Warning(err)
	}

	runXrdbMerge := func(filePath string) {
		err := exec.Command(xrdbBin, "-merge", filePath).Run()
		if err != nil {
			logger.Warningf("xrdb merge %s failed: %v", filePath, err)
		}
	}

	for _, fileInfo := range fileInfos {
		filePath := filepath.Join(sysResourcesDir, fileInfo.Name())
		runXrdbMerge(filePath)
	}
	userResources := filepath.Join(_homeDir, ".Xresources")
	fileInfo, err := os.Stat(userResources)
	if err == nil && !fileInfo.IsDir() {
		runXrdbMerge(userResources)
	}
}

// 35x11-common_xhost-local
func runScript35X11CommonXHostLocalFaster(xConn *x.Conn) {
	u, err := user.Current()
	if err != nil {
		logger.Warning("get current user failed:", err)
		return
	}
	// family 为 5 是 Server Interpreted 的意思
	err = x.ChangeHostsChecked(xConn, x.HostModeInsert, 5, "localuser\x00"+u.Username).Check(xConn)
	if err != nil {
		logger.Warning("change x hosts failed:", err)
	}
}

// 40x11-common_xsessionrc 读取 $HOME/.xsessionrc, 一般没有这个文件，暂不实现

// 55numlockx 这个功能由 dde-session-daemon 实现了，它记录并设置 numlock 的状态。

// 60imwheel_start-imwheel 这个功能也由 dde-session-daemon 实现了

// 70im-config_launch
func runScript70ImConfigLaunchFaster() {
	// 一般使用 fcitx
	_envVars["XMODIFIERS"] = "@im=fcitx"
	_envVars["GTK_IM_MODULE"] = "fcitx"
	_envVars["CLUTTER_IM_MODULE"] = "fcitx"
	_envVars["QT_IM_MODULE"] = "fcitx"
	_envVars["QT4_IM_MODULE"] = "fcitx"
}

// /etc/X11/Xsession.d/75dbus_dbus-launch 已经设置了 DBUS_SESSION_BUS_ADDRESS 环境变量，就不会执行 dbus-launch 了。

// 90gpg-agent
func runScript90GpgAgentFaster() {
	// NOTE: 放在启动核心组件之后

	gpgConfCmd, err := exec.LookPath("gpgconf")
	if err != nil {
		return
	}
	// gpgconf --list-dirs agent-socket
	out, err := exec.Command(gpgConfCmd, "--list-dirs", "agent-socket").Output()
	if err != nil {
		logger.Warning("get gpg agent socket failed:", err)
		return
	}
	gpgAgentInfo := string(bytes.TrimSpace(out)) + ":0:1"
	_envVars["GPG_AGENT_INFO"] = gpgAgentInfo
	_ = os.Setenv("GPG_AGENT_INFO", gpgAgentInfo)

	// gpgconf --list-options gpg-agent
	out, err = exec.Command(gpgConfCmd, "--list-options", "gpg-agent").Output()
	if err != nil {
		logger.Warning("list gpg agent options failed:", err)
		return
	}
	lines := bytes.Split(out, []byte{'\n'})
	for _, line := range lines {
		if !bytes.HasPrefix(line, []byte("enable-ssh-support:")) {
			continue
		}
		fields := bytes.Split(line, []byte{':'})
		if len(fields) < 10 {
			continue
		}
		if len(bytes.TrimSpace(fields[9])) > 0 {
			// enable ssh support
			// gpgconf --list-dirs agent-ssh-socket
			out, err = exec.Command(gpgConfCmd, "--list-dirs", "agent-ssh-socket").Output()
			if err != nil {
				logger.Warning("get gpg agent ssh socket failed:", err)
				return
			}

			sshAuthSock := string(bytes.TrimSpace(out))
			_envVars["SSH_AUTH_SOCK"] = sshAuthSock
			_ = os.Setenv("SSH_AUTH_SOCK", sshAuthSock)
			return
		}
	}
}

// 90qt-ally
func runScript90QtA11yFaster() {
	_envVars["QT_ACCESSIBILITY"] = "1"
}

// 90x11-common_ssh-agent 默认选项是 no-use-ssh-agent 不使用此功能

// 95dbus_update-activation-env 已经在 startdde 中实现

func runXSessionScriptsFaster(xConn *x.Conn) {
	runScript00DeepinDdeEnvFaster()
	runScript05UosProfileFaster()
	runScript20DBusXdgRuntimeFaster()
	runScript35X11CommonXHostLocalFaster(xConn)
	runScript70ImConfigLaunchFaster()
	runScript90QtA11yFaster()
}
