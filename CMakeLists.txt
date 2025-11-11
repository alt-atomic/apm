cmake_minimum_required(VERSION 3.31)
project(apm)

set(CMAKE_CXX_STANDARD 14)

include_directories(internal/common/binding/apt/lib)

add_executable(apm
        internal/common/binding/apt/lib/apt_dist.cpp
        internal/common/binding/apt/lib/apt_getinfo.cpp
        internal/common/binding/apt/lib/apt_install.cpp
        internal/common/binding/apt/lib/apt_internal.h
        internal/common/binding/apt/lib/apt_package_operations.cpp
        internal/common/binding/apt/lib/apt_package_operations.h
        internal/common/binding/apt/lib/apt_search.cpp
        internal/common/binding/apt/lib/apt_simulate.cpp
        internal/common/binding/apt/lib/apt_wrapper.cpp
        internal/common/binding/apt/lib/apt_wrapper.h)
