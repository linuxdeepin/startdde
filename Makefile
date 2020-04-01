PREFIX = /usr
GOPATH_DIR = gopath
GOPKG_PREFIX = pkg.deepin.io/dde/startdde
GOBUILD = go build -v

all: build

prepare:
	@if [ ! -d ${GOPATH_DIR}/src/${GOPKG_PREFIX} ]; then \
		mkdir -p ${GOPATH_DIR}/src/$(dir ${GOPKG_PREFIX}); \
		ln -sf ../../../.. ${GOPATH_DIR}/src/${GOPKG_PREFIX}; \
		fi

auto_launch_json:
ifdef AUTO_LAUNCH_CHINESE
	cp misc/auto_launch/chinese.json misc/config/auto_launch.json
else
	cp misc/auto_launch/default.json misc/config/auto_launch.json
endif
	# check validity
	jq . misc/config/auto_launch.json >/dev/null

startdde:
	env GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}" ${GOBUILD} -o startdde

fix-xauthority-perm:
	env GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}" ${GOBUILD} -o fix-xauthority-perm ${GOPKG_PREFIX}/cmd/fix-xauthority-perm

greeter-display-daemon:
	env GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}" ${GOBUILD} -o greeter-display-daemon ${GOPKG_PREFIX}/cmd/greeter-display-daemon

build: prepare startdde auto_launch_json fix-xauthority-perm greeter-display-daemon

test: prepare
	env GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}" go test -v ./...

install:
	install -Dm755 startdde ${DESTDIR}${PREFIX}/bin/startdde
	mkdir -p ${DESTDIR}${PREFIX}/share/xsessions
	@for i in $(shell ls misc/xsessions/ | grep -E '*.in$$' );do sed 's|@PREFIX@|$(PREFIX)|g' misc/xsessions/$$i > ${DESTDIR}${PREFIX}/share/xsessions/$${i%.in}; done
	install -Dm755 fix-xauthority-perm ${DESTDIR}${PREFIX}/sbin/deepin-fix-xauthority-perm
	install -Dm755 greeter-display-daemon ${DESTDIR}${PREFIX}/lib/deepin-daemon/greeter-display-daemon
	install -Dm644 misc/lightdm.conf ${DESTDIR}${PREFIX}/share/lightdm/lightdm.conf.d/60-deepin.conf
	mkdir -p ${DESTDIR}${PREFIX}/share/startdde/
	cp -f misc/config/* ${DESTDIR}${PREFIX}/share/startdde/
	mkdir -p ${DESTDIR}/etc/X11/Xsession.d/
	cp -f misc/Xsession.d/* ${DESTDIR}/etc/X11/Xsession.d/
	mkdir -p ${DESTDIR}/etc/profile.d/
	cp -f misc/profile.d/* ${DESTDIR}/etc/profile.d/

clean:
	rm -rf ${GOPATH_DIR}
	rm -f startdde
	rm -f fix-xauthority-perm
	rm -f greeter-display-daemon

rebuild: clean build

check_code_quality: prepare
	env GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}" go vet ./...
