%global with_debug 1
%global _unpackaged_files_terminate_build 0
%if 0%{?with_debug}
%global debug_package   %{nil}
%endif

Name:           startdde
Version:        5.4.0.1
Release:        1
Summary:        Starter of deepin desktop environment
License:        GPLv3
URL:            https://github.com/linuxdeepin/startdde
Source0:        %{name}_%{version}.orig.tar.xz


BuildRequires:  golang jq
#BuildRequires:  deepin-gir-generator
#BuildRequires:  golang-github-linuxdeepin-go-dbus-factory-devel
#BuildRequires:  go-lib-devel
#BuildRequires:  golang-github-linuxdeepin-go-x11-client-devel
BuildRequires:  golang-github-davecgh-go-spew-devel
BuildRequires:  gocode >= 0.0.0.1
#BuildRequires:  golang-golang-org-net-devel
BuildRequires:  golang-github-cryptix-wav-devel
BuildRequires:  golang-golang-x-xerrors-devel
#BuildRequires:  golang-github-linuxdeepin-go-x11-client-devel
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
%autosetup
patch Makefile < rpm/Makefile.patch
patch misc/auto_launch/chinese.json < rpm/chinese.json.patch
patch misc/auto_launch/default.json < rpm/default.json.patch

%build
export GOPATH="%{gopath}"
make GO_BUILD_FLAGS=-trimpath

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
%{_bindir}/%{name}
#%{_sbindir}/deepin-session
%{_sbindir}/deepin-fix-xauthority-perm
%{_datadir}/xsessions/deepin.desktop
%{_datadir}/lightdm/lightdm.conf.d/60-deepin.conf
%{_datadir}/%{name}/auto_launch.json
%{_datadir}/%{name}/memchecker.json
/usr/lib/deepin-daemon/greeter-display-daemon

%changelog
* Fri Jun 12 2020 uoser <uoser@uniontech.com> - 5.4.0.1
- Update to 5.4.0.1

