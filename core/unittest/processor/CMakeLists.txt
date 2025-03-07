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
project(processor_unittest)

add_executable(processor_filter_unittest LogFilterUnittest.cpp)
target_link_libraries(processor_filter_unittest unittest_base)

add_executable(processor_split_log_string_native_unittest ProcessorSplitLogStringNativeUnittest.cpp)
target_link_libraries(processor_split_log_string_native_unittest unittest_base)

add_executable(processor_split_regex_native_unittest ProcessorSplitRegexNativeUnittest.cpp)
target_link_libraries(processor_split_regex_native_unittest unittest_base)

add_executable(processor_parse_regex_native_unittest ProcessorParseRegexNativeUnittest.cpp)
target_link_libraries(processor_parse_regex_native_unittest unittest_base)

add_executable(processor_parse_timestamp_native_unittest ProcessorParseTimestampNativeUnittest.cpp)
target_link_libraries(processor_parse_timestamp_native_unittest unittest_base)

add_executable(processor_fill_group_info_native_unittest ProcessorFillGroupInfoNativeUnittest.cpp)
target_link_libraries(processor_fill_group_info_native_unittest unittest_base)