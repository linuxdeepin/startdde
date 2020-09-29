%if 0%{?fedora} == 0
%global without_debug 1
%endif
%if 0%{?without_debug}
%global debug_package   %{nil}
%endif

Name:           startdde
Version:        5.6.0.30
Release:        1%{?fedora:%dist}
Summary:        Starter of deepin desktop environment
License:        GPLv3
URL:            https://github.com/linuxdeepin/startdde
%if 0%{?fedora}
Source0:        %{url}/archive/%{version}/%{name}-%{version}.tar.gz
ExclusiveArch:  %{?go_arches:%{go_arches}}%{!?go_arches:%{ix86} x86_64 %{arm}}
%else
Source0:        %{name}_%{version}.orig.tar.xz
%endif

BuildRequires:  golang jq
%if 0%{?fedora}
BuildRequires:  golang(pkg.deepin.io/dde/api/dxinput)
BuildRequires:  golang(pkg.deepin.io/lib)
BuildRequires:  golang(github.com/godbus/dbus)
BuildRequires:  golang(github.com/linuxdeepin/go-x11-client)
BuildRequires:  golang(github.com/davecgh/go-spew/spew)
BuildRequires:  golang(golang.org/x/xerrors)
BuildRequires:  systemd-rpm-macros
Requires:       deepin-daemon
Requires:       deepin-session-shell
%else
BuildRequires:  golang-github-davecgh-go-spew-devel
BuildRequires:  gocode >= 0.0.0.1
BuildRequires:  golang-golang-x-xerrors-devel
%endif
BuildRequires:  golang-github-cryptix-wav-devel
BuildRequires:  glib2-devel
BuildRequires:  pkgconfig(x11)
BuildRequires:  libXcursor-devel
BuildRequires:  libXfixes-devel
BuildRequires:  gtk3-devel
BuildRequires:  pulseaudio-libs-devel
BuildRequires:  libgnome-keyring-devel
BuildRequires:  alsa-lib-devel
BuildRequires:  pkgconfig(gudev-1.0)

%description
Startdde is used for launching DDE components and invoking user's
custom applications which compliant with xdg autostart specification.

%prep
%autosetup -p1
# fix path, derived from 6a49e9a
sed -i 's:X11/Xsession.d:X11/xinit/xinitrc.d:' Makefile
# fix deepin-daemon executables path
find * -type f -not -path "rpm/*" -print0 | xargs -0 sed -i 's:/lib/deepin-daemon/:/libexec/deepin-daemon/:'
# fix dde-polkit-agent path
sed -i '/polkit/s|lib|libexec|' watchdog/dde_polkit_agent.go

%build
export GOPATH="%{gopath}"
%if 0%{?without_debug}
GO_BUILD_FLAGS="-trimpath"
%endif
BUILD_ID="0x$(head -c20 /dev/urandom|od -An -tx1|tr -d ' \n')"
%make_build GOBUILD="go build -compiler gc -ldflags \"${LDFLAGS} -B $BUILD_ID\" -a $GO_BUILD_FLAGS -v -x"
# rebuild other executables with different build-ids
for cmd in fix-xauthority-perm greeter-display-daemon; do
    rm $cmd
    BUILD_ID="0x$(head -c20 /dev/urandom|od -An -tx1|tr -d ' \n')"
    %make_build $cmd GOBUILD="go build -compiler gc -ldflags \"${LDFLAGS} -B $BUILD_ID\" -a $GO_BUILD_FLAGS -v -x"
done

%install
%make_install

%post
%systemd_post dde-readahead.service

%preun
%systemd_preun dde-readahead.service

%postun
%systemd_postun_with_restart dde-readahead.service

%files
%doc README.md
%license LICENSE
%{_sysconfdir}/X11/xinit/xinitrc.d/00deepin-dde-env
%{_sysconfdir}/X11/xinit/xinitrc.d/01deepin-profile
%{_sysconfdir}/profile.d/deepin-xdg-dir.sh
%{_bindir}/%{name}
%{_sbindir}/deepin-fix-xauthority-perm
%{_datadir}/xsessions/deepin.desktop
%{_datadir}/lightdm/lightdm.conf.d/60-deepin.conf
%{_datadir}/%{name}/
%{_libexecdir}/deepin-daemon/greeter-display-daemon

%changelog
* Fri Jun 12 2020 uoser <uoser@uniontech.com> - 5.4.0.1
- Update to 5.4.0.1
