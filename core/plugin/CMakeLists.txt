# Copyright 2022 iLogtail Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

cmake_minimum_required(VERSION 2.9)
project(plugin)

file(GLOB LIB_SOURCE_FILES *.cpp *.h)
list(REMOVE_ITEM LIB_SOURCE_FILES ${CMAKE_CURRENT_SOURCE_DIR}/LogtailPluginAdapter.h)
list(REMOVE_ITEM LIB_SOURCE_FILES ${CMAKE_CURRENT_SOURCE_DIR}/LogtailPluginAdapter.cpp)
append_source_files(LIB_SOURCE_FILES)
add_library(${PROJECT_NAME} STATIC ${LIB_SOURCE_FILES})
target_link_libraries(${PROJECT_NAME} common)
target_link_libraries(${PROJECT_NAME} logger)
target_link_libraries(${PROJECT_NAME} config_manager)
target_link_libraries(${PROJECT_NAME} sender)
target_link_libraries(${PROJECT_NAME} monitor)
target_link_libraries(${PROJECT_NAME} app_config)
target_link_libraries(${PROJECT_NAME} config)
if (UNIX)
    target_link_libraries(${PROJECT_NAME} dl)
endif ()

add_library(PluginAdapter SHARED LogtailPluginAdapter.h LogtailPluginAdapter.cpp)
if (UNIX)
    target_link_libraries(PluginAdapter dl)
elseif (MSVC)
    add_definitions(-DPLUGIN_ADAPTER_EXPORTS)
endif ()