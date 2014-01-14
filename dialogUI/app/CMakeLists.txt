#add_subdirectory(greeter)
add_subdirectory(shutdown)

install(
    DIRECTORY ${PROJECT_SOURCE_DIR}/resources/common
    DESTINATION ${RESOURCE_DIR}
    )
