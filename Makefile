PREFIX = /usr
GOPATH_DIR = gopath
GOPKG_PREFIX = pkg.deepin.io/dde/startdde

ifndef USE_GCCGO
    GOBUILD = go build
else
    LDFLAGS = $(shell pkg-config --libs gio-2.0 gdk-3.0 gdk-pixbuf-xlib-2.0 x11)
    GOBUILD = go build -compiler gccgo -gccgoflags "${LDFLAGS}"
endif

all: build

prepare:
	@if [ ! -d ${GOPATH_DIR}/src/${GOPKG_PREFIX} ]; then \
		mkdir -p ${GOPATH_DIR}/src/$(dir ${GOPKG_PREFIX}); \
		ln -sf ../../../.. ${GOPATH_DIR}/src/${GOPKG_PREFIX}; \
	fi

startdde:
	env GOPATH="${GOPATH}:${CURDIR}/${GOPATH_DIR}" ${GOBUILD} -o startdde

dialog:
	echo "Start Building dialogUI"
	cd dialogUI && mkdir build
	cd dialogUI/build && cmake .. -DCMAKE_INSTALL_PREFIX=${PREFIX} -DCMAKE_BUILD_TYPE=Release

build: prepare startdde dialog

install:
	mkdir -p ${DESTDIR}${PREFIX}/bin && cp startdde ${DESTDIR}${PREFIX}/bin
	mkdir -pv ${DESTDIR}${PREFIX}/share/glib-2.0/schemas
	cp misc/schemas/*.xml ${DESTDIR}${PREFIX}/share/glib-2.0/schemas/
	mkdir -p ${DESTDIR}${PREFIX}/share/xsessions && cp misc/*.desktop ${DESTDIR}${PREFIX}/share/xsessions
	echo "Install dialogUI"
	cd dialogUI/build && make DESTDIR=${DESTDIR} install

clean:
	rm -rf ${GOPATH_DIR}
	rm -rf dialogUI/build

rebuild: clean build
