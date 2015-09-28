PREFIX = /usr
GOPATH_DIR = gopath
GOPKG_PREFIX = pkg.deepin.io/dde/startdde

ifndef USE_GCCGO
	ifndef GOLANG_DEBUG
		GOLDFLAGS = -ldflags '-s -w'
	endif

	GOBUILD = go build ${GOLDFLAGS}
else
	ifndef GOLANG_DEBUG
		GOLDFLAGS = -s -w -Os -O2
	endif

	GOLDFLAGS += $(shell pkg-config --libs gio-2.0 gtk+-3.0 gdk-pixbuf-xlib-2.0 x11)
	GOBUILD = go build -compiler gccgo -gccgoflags "${GOLDFLAGS}"
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
	@echo "Start Building dialogUI"
	cd dialogUI && mkdir build
	cd dialogUI/build && cmake .. -DCMAKE_INSTALL_PREFIX=${PREFIX} -DCMAKE_BUILD_TYPE=Release

build: prepare startdde dialog

install:
	mkdir -p ${DESTDIR}${PREFIX}/bin
	install -Dm755 startdde ${DESTDIR}${PREFIX}/bin/startdde
	sed 's|@PREFIX@|$(PREFIX)|g' misc/bin/startdde-2D.in >  ${DESTDIR}${PREFIX}/bin/startdde-2D
	chmod 0755 ${DESTDIR}${PREFIX}/bin/startdde-2D
	mkdir -p ${DESTDIR}${PREFIX}/share/xsessions
	@for i in $(shell ls misc/xsessions/ | grep -E '*.in$$' );do sed 's|@PREFIX@|$(PREFIX)|g' misc/xsessions/$$i > ${DESTDIR}${PREFIX}/share/xsessions/$${i%.in}; done
	@echo "Install dialogUI"
	cd dialogUI/build && make DESTDIR=${DESTDIR} install

clean:
	rm -rf ${GOPATH_DIR}
	rm -rf dialogUI/build

rebuild: clean build
