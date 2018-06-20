# Startdde

**Description**:
Startdde is used for launching DDE components and invoking
user's custom applications which compliant with xdg autostart specification.

## Dependencies

### Build dependencies

- cmake
- pkg-config
- golang-go
- [go-dlib](https://github.com/linuxdeepin/go-lib)
- [go-fsnotify](https://github.com/howeyc/fsnotify)
- [dde-dbus-factory](https://github.com/linuxdeepin/dbus-factory)
- [go-gir-generator](https://github.com/linuxdeepin/go-gir-generator)
- [dde-api](https://github.com/linuxdeepin/dde-api)
- [go-x11-client](https://github.com/linuxdeepin/go-x11-client)
- libgnome-keyring
- libxfixes
- libxcursor

### Runtime dependencies

- dde-daemon
- deepin-wm | deepin-metacity
- libgnome-keyring
- libxfixes
- libxcursor

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
* [WiKi](https://wiki.deepin.org/)

## Getting involved

We encourage you to report issues and contribute changes

* [Contribution guide for developers](https://github.com/linuxdeepin/developer-center/wiki/Contribution-Guidelines-for-Developers-en). (English)
* [开发者代码贡献指南](https://github.com/linuxdeepin/developer-center/wiki/Contribution-Guidelines-for-Developers) (中文)

## License

Startdde is licensed under [GPLv3](LICENSE).
