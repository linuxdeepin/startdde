# Startdde

**描述**:
startdde 用于启动DDE组件和调用
符合xdg自动启动规范的用户自定义应用程序。

## 依赖

### 编译依赖

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

### 运行依赖

- dde-daemon
- deepin-wm | deepin-metacity
- libgnome-keyring
- libxfixes
- libxcursor

## 安装

### Deepin

startdde需要预安装以下包
```
$ sudo apt-get build-dep startdde
```

构建
```
$ GOPATH=/usr/share/gocode make
```

如果你有独立的测试构建环境（比如一个 docker 容器），你可以直接安装它
```
$ sudo make install
```

生成包文件并使用它安装startdde
```
$ debuild -uc -us ...
$ sudo dpkg -i ../startdde-*deb
```

## 使用方法

使用以下命令运行 Startdde

```
Usage of /usr/bin/startdde:
  -d=false: debug
```


### 无需显示管理器直接运行

```
$ echo "dbus-launch --exit-with-session /usr/bin/startdde" > ~/.xinitrc
$ startx
```

### 使用显示管理器运行

1. 在`/usr/share/xsessions`中构建会话桌面

```
cat /usr/share/xsessions/deepin.desktop

[Desktop Entry]
Name=Deepin
Comment=Deepin Desktop Environment
Exec=/usr/bin/startdde
```

2. 使用相应的显示管理器如gdm, kdm或者lightdm来启动startdde

## 获得帮助

如果您遇到任何其他问题，您可能还会发现这些渠道很有用：

* [Gitter](https://gitter.im/orgs/linuxdeepin/rooms)
* [IRC channel](https://webchat.freenode.net/?channels=deepin)
* [Forum](https://bbs.deepin.org)
* [WiKi](https://wiki.deepin.org/)

## 贡献指南

我们鼓励您报告问题并做出更改。

* [Contribution guide for developers](https://github.com/linuxdeepin/developer-center/wiki/Contribution-Guidelines-for-Developers-en). (English)
* [开发者代码贡献指南](https://github.com/linuxdeepin/developer-center/wiki/Contribution-Guidelines-for-Developers) (中文)

## 开源协议

startdde项目在LGPL-3.0-or-later开源协议下发布。
