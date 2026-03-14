cmake_minimum_required(VERSION 3.31)
project(apm)

set(CMAKE_CXX_STANDARD 17)

set(LIB_DIR internal/common/binding/apt/lib)

include_directories(
        ${LIB_DIR}/include
        ${LIB_DIR}/src
)

add_executable(apm
        ${LIB_DIR}/include/apt.h
        ${LIB_DIR}/include/apt_common.h
        ${LIB_DIR}/include/apt_cache.h
        ${LIB_DIR}/include/apt_config.h
        ${LIB_DIR}/include/apt_ext_rpm.h
        ${LIB_DIR}/include/apt_lock.h
        ${LIB_DIR}/include/apt_logging.h
        ${LIB_DIR}/include/apt_package.h
        ${LIB_DIR}/include/apt_system.h
        ${LIB_DIR}/include/apt_transaction.h

        ${LIB_DIR}/src/internal.h
        ${LIB_DIR}/src/error.h
        ${LIB_DIR}/src/progress.h
        ${LIB_DIR}/src/changes.h
        ${LIB_DIR}/src/dependency.h
        ${LIB_DIR}/src/executor.h
        ${LIB_DIR}/src/ext_rpm.h
        ${LIB_DIR}/src/filelist.h
        ${LIB_DIR}/src/mark.h
        ${LIB_DIR}/src/package_fill.h
        ${LIB_DIR}/src/planner.h
        ${LIB_DIR}/src/resolve.h

        ${LIB_DIR}/apt_cache.cpp
        ${LIB_DIR}/apt_changes.cpp
        ${LIB_DIR}/apt_config.cpp
        ${LIB_DIR}/apt_dependency.cpp
        ${LIB_DIR}/apt_error.cpp
        ${LIB_DIR}/apt_executor.cpp
        ${LIB_DIR}/apt_ext_rpm.cpp
        ${LIB_DIR}/apt_filelist.cpp
        ${LIB_DIR}/apt_lock.cpp
        ${LIB_DIR}/apt_logging.cpp
        ${LIB_DIR}/apt_mark.cpp
        ${LIB_DIR}/apt_planner.cpp
        ${LIB_DIR}/apt_progress.cpp
        ${LIB_DIR}/apt_package.cpp
        ${LIB_DIR}/apt_package_fill.cpp
        ${LIB_DIR}/apt_packages.cpp
        ${LIB_DIR}/apt_resolve.cpp
        ${LIB_DIR}/apt_system.cpp
        ${LIB_DIR}/apt_transaction.cpp)
