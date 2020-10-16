%global _missing_build_ids_terminate_build 0
%global debug_package   %{nil}

%define specrelease 2%{?dist}
%if 0%{?openeuler}
%define specrelease 2
%endif

Name:           startdde
Version:        5.6.0.7
Release:        %{specrelease}
Summary:        Starter of deepin desktop environment
License:        GPLv3
URL:            https://github.com/linuxdeepin/startdde
Source0:        %{name}-%{version}.tar.xz

BuildRequires:  golang
BuildRequires:  jq
BuildRequires:  gocode
BuildRequires:  glib2-devel
BuildRequires:  pkgconfig(x11)
BuildRequires:  libXcursor-devel
BuildRequires:  libXfixes-devel
BuildRequires:  gtk3-devel
BuildRequires:  pulseaudio-libs-devel
BuildRequires:  libgnome-keyring-devel
BuildRequires:  alsa-lib-devel
BuildRequires:  pkgconfig(gudev-1.0)

Provides:       x-session-manager
Requires:       dde-daemon
Requires:       procps
Requires:       gocode
Requires:       deepin-desktop-schemas
Requires:       dde-kwin
Requires:       libXfixes
Requires:       libXcursor
Recommends:     dde-qt5integration

%description
%{summary}.

%prep
%autosetup -n %{name}-%{version}
patch Makefile < rpm/Makefile.patch
patch misc/auto_launch/chinese.json < rpm/chinese.json.patch
patch misc/auto_launch/default.json < rpm/default.json.patch
patch main.go < rpm/main.go.patch

%build
export GOPATH=/usr/share/gocode

## Scripts in /etc/X11/Xsession.d are not executed after xorg start
sed -i 's|X11/Xsession.d|X11/xinit/xinitrc.d|g' Makefile

%make_build GO_BUILD_FLAGS=-trimpath

%install
%make_install

%post
xsOptsFile=/etc/X11/Xsession.options
update-alternatives --install /usr/bin/x-session-manager x-session-manager \
    /usr/bin/startdde 90 || true
if [ -f $xsOptsFile ];then
	sed -i '/^use-ssh-agent/d' $xsOptsFile
	if ! grep '^no-use-ssh-agent' $xsOptsFile >/dev/null; then
		echo no-use-ssh-agent >> $xsOptsFile
	fi
fi

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
%{_datadir}/%{name}/auto_launch.json
%{_datadir}/%{name}/memchecker.json
/usr/lib/deepin-daemon/greeter-display-daemon

%changelog
* Wed Oct 14 2020 guoqinglan <guoqinglan@uniontech.com> - 5.6.0.5-2
- bugfix-49318, modify /usr/lib/deepin-daemon path

* Sat Oct 10 2020 guoqinglan <guoqinglan@uniontech.com> - 5.6.0.5-1
- bugfix-49970, fix post add preun scripts
