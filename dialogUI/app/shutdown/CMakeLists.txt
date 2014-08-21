SET(NAME "shutdown")

GEN_JSC_SOURCE2(JSC_GEN "jsc" "shutdown" "shutdown.cfg" "dbus.cfg")

include_directories(${DWEBVIEW_INCLUDE_DIRS})

SET(SRC_LIST  ${JSC_GEN} )

add_executable(${NAME} ${SRC_LIST} shutdown.c)

target_link_libraries(${NAME} dcom dbus ${DWEBVIEW_LIBRARIES})
add_dependencies(${NAME} ${NAME}_js)

SET_TARGET_PROPERTIES(${NAME}
    PROPERTIES OUTPUT_NAME dde-${NAME}
    )

execute_process(
    COMMAND make
    WORKING_DIRECTORY ${PROJECT_SOURCE_DIR}/resources/${NAME}/js
    )

install(
    PROGRAMS ${PROJECT_BINARY_DIR}/dde-${NAME}
    DESTINATION bin
    )

install(
    DIRECTORY ${PROJECT_SOURCE_DIR}/resources/${NAME}
    DESTINATION ${RESOURCE_DIR}
    )
 
GEN_JS_FILE(${NAME})
