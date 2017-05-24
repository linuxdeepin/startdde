# Startdde

**Description**:
Startdde is used for launching DDE components and invoking
user's custom applications which compliant with xdg autostart specification.

## Dependencies

### Build dependencies

- cmake
- pkg-config
- golang-go
- coffeescript
- [go-dlib](https://github.com/linuxdeepin/go-lib)
- [go-xgb](https://github.com/BurntSushi/xgb)
- [go-xgbutil](https://github.com/BurntSushi/xgbutil)
- [go-fsnotify](https://github.com/howeyc/fsnotify)
- [dde-dbus-factory](https://github.com/linuxdeepin/dbus-factory)
- [go-gir-generator](https://github.com/linuxdeepin/go-gir-generator)
* [dde-api](https://github.com/linuxdeepin/dde-api)

### Runtime dependencies

- dde-daemon
- deepin-wm-switcher
- deepin-wm | deepin-metacity

## Installation

### Deepin

Install prerequisites
```
$ sudo apt-get build-dep startdde
```

Build
```
$ GOPATH=/usr/share/gocode make
```

If you have isolated testing build environment (say a docker container), you can install it directly
```
$ sudo make install
```

generate package files and install Startdde with it
```
$ debuild -uc -us ...
$ sudo dpkg -i ../startdde-*deb
```

## Usage

Run Startdde with the command below

```
Usage of /usr/bin/startdde:
  -d=false: debug
  -wm="/usr/bin/deepin-wm-switcher": the window manager used by dde
```


### Directly run without display manager

```
$ echo "dbus-launch --exit-with-session /usr/bin/startdde" > ~/.xinitrc
$ startx
```

### Run with display manager

1. construct a session desktop in /usr/share/xsessions

```
cat /usr/share/xsessions/deepin.desktop

[Desktop Entry]
Name=Deepin
Comment=Deepin Desktop Environment
Exec=/usr/bin/startdde
```

2. Using DisplayManager like, gdm, kdm or lightdm to startup Startdde

## Getting help

Any usage issues can ask for help via

* [Gitter](https://gitter.im/orgs/linuxdeepin/rooms)
* [IRC channel](https://webchat.freenode.net/?channels=deepin)
* [Forum](https://bbs.deepin.org)
* [WiKi](http://wiki.deepin.org/)

## Getting involved

We encourage you to report issues and contribute changes

* [Contribution guide for users](http://wiki.deepin.org/index.php?title=Contribution_Guidelines_for_Users)
* [Contribution guide for developers](http://wiki.deepin.org/index.php?title=Contribution_Guidelines_for_Developers)

## License

Startdde is licensed under [GPLv3](LICENSE).
