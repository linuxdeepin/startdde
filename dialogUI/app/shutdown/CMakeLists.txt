SET(NAME "dde-shutdown")
SET(NAME_SRC "shutdown")

GEN_JSC_SOURCE2(JSC_GEN "jsc" "shutdown" "shutdown.cfg" "dbus.cfg")

include_directories(${DWEBVIEW_INCLUDE_DIRS})

SET(SRC_LIST  ${JSC_GEN} )

add_executable(${NAME} ${SRC_LIST} bg.c shutdown.c)

target_link_libraries(${NAME} dcom dbus ${DWEBVIEW_LIBRARIES})
add_dependencies(${NAME} ${NAME_SRC}_js)

execute_process(
    COMMAND make
    WORKING_DIRECTORY ${PROJECT_SOURCE_DIR}/resources/${NAME_SRC}/js
    )

install(
    PROGRAMS ${PROJECT_BINARY_DIR}/${NAME}
    DESTINATION bin
    )

install(
    DIRECTORY ${PROJECT_SOURCE_DIR}/resources/${NAME_SRC}
    DESTINATION ${RESOURCE_DIR}
    )

install(
    FILES ${PROJECT_SOURCE_DIR}/app/shutdown/com.deepin.dde.shutdown.service
    DESTINATION /usr/share/dbus-1/services/
    )
 
GEN_JS_FILE(${NAME_SRC})
