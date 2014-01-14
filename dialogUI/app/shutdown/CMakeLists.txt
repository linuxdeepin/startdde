SET(NAME "shutdown")

GEN_JSC_SOURCE2(JSC_GEN "jsc" "shutdown" "shutdown.cfg" "dbus.cfg")

pkg_check_modules(LIGHTDM REQUIRED liblightdm-gobject-1)
pkg_check_modules(OPENCV REQUIRED opencv)
pkg_check_modules(GST REQUIRED gstreamer-0.10)

include_directories(${DWEBVIEW_INCLUDE_DIRS} ${LIGHTDM_INCLUDE_DIRS}
    ${OPENCV_INCLUDE_DIRES} ${GST_INCLUDE_DIRS})

SET(SRC_LIST  ${JSC_GEN} )

add_executable(dshutdown ${SRC_LIST} shutdown.c)

set(OPENCV_LIBS opencv_core opencv_imgproc opencv_objdetect)
target_link_libraries(dshutdown dcom dbus ${DWEBVIEW_LIBRARIES} ${OPENCV_LIBS} ${GST_LIBRARIES})


execute_process(
    COMMAND make
    WORKING_DIRECTORY ${PROJECT_SOURCE_DIR}/resources/${NAME}/js
    )

install(
    PROGRAMS ${PROJECT_BINARY_DIR}/dshutdown
    DESTINATION lib/deepin-daemon
    )

install(
    DIRECTORY ${PROJECT_SOURCE_DIR}/resources/${NAME}
    DESTINATION ${RESOURCE_DIR}
    )

install(
    FILES ${PROJECT_SOURCE_DIR}/app/shutdown/com.deepin.dde.shutdown.service
    DESTINATION /usr/share/dbus-1/services/
    )

