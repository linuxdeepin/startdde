PREFIX = /usr
GOPATH_DIR = gopath
GOPKG_PREFIX = pkg.deepin.io/dde/startdde
GOBUILD = go build

ifdef USE_GCCGO
	GOLDFLAGS = -Os -O2
	GOLDFLAGS += $(shell pkg-config --libs gio-2.0 gtk+-3.0 gdk-pixbuf-xlib-2.0 x11 xi libpulse-simple alsa gnome-keyring-1 xfixes xcursor)
	GOLDFLAGS += -lm
	GOBUILD += -compiler gccgo -gccgoflags "${GOLDFLAGS}"
endif

all: build

prepare:
	@if [ ! -d ${GOPATH_DIR}/src/${GOPKG_PREFIX} ]; then \
		mkdir -p ${GOPATH_DIR}/src/$(dir ${GOPKG_PREFIX}); \
		ln -sf ../../../.. ${GOPATH_DIR}/src/${GOPKG_PREFIX}; \
		fi

startdde:
	env GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}" ${GOBUILD} -v -o startdde

build: prepare startdde

install:
	install -Dm755 startdde ${DESTDIR}${PREFIX}/bin/startdde
	mkdir -p ${DESTDIR}${PREFIX}/share/xsessions
	@for i in $(shell ls misc/xsessions/ | grep -E '*.in$$' );do sed 's|@PREFIX@|$(PREFIX)|g' misc/xsessions/$$i > ${DESTDIR}${PREFIX}/share/xsessions/$${i%.in}; done
	install -Dm755 misc/deepin-session ${DESTDIR}${PREFIX}/sbin/deepin-session
	install -Dm644 misc/lightdm.conf ${DESTDIR}${PREFIX}/share/lightdm/lightdm.conf.d/60-deepin.conf
	mkdir -p ${DESTDIR}${PREFIX}/share/startdde/
	cp -f misc/config/* ${DESTDIR}${PREFIX}/share/startdde/
	mkdir -p ${DESTDIR}/share/startdde/
	cp -f misc/config/* ${DESTDIR}/share/startdde/

clean:
	-rm -rf ${GOPATH_DIR}
	-rm -f startdde

rebuild: clean build
