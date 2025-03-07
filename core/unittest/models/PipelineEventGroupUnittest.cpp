// Copyright 2023 iLogtail Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#include <cstdlib>
#include "unittest/Unittest.h"

#include "common/JsonUtil.h"
#include "models/PipelineEventGroup.h"

namespace logtail {

class PipelineEventGroupUnittest : public ::testing::Test {
public:
    void SetUp() override {
        mSourceBuffer.reset(new SourceBuffer);
        mEventGroup.reset(new PipelineEventGroup(mSourceBuffer));
    }

    void TestSwapEvents();
    void TestSetMetadata();
    void TestDelMetadata();
    void TestFromJsonToJson();

protected:
    std::shared_ptr<SourceBuffer> mSourceBuffer;
    std::unique_ptr<PipelineEventGroup> mEventGroup;
};

APSARA_UNIT_TEST_CASE(PipelineEventGroupUnittest, TestSwapEvents, 0);
APSARA_UNIT_TEST_CASE(PipelineEventGroupUnittest, TestSetMetadata, 0);
APSARA_UNIT_TEST_CASE(PipelineEventGroupUnittest, TestDelMetadata, 0);
APSARA_UNIT_TEST_CASE(PipelineEventGroupUnittest, TestFromJsonToJson, 0);

void PipelineEventGroupUnittest::TestSwapEvents() {
    PipelineEventPtr logEventPtr(LogEvent::CreateEvent(mSourceBuffer));
    PipelineEventPtr metricEventPtr(MetricEvent::CreateEvent(mSourceBuffer));
    PipelineEventPtr spanEventPtr(SpanEvent::CreateEvent(mSourceBuffer));
    mEventGroup->AddEvent(logEventPtr);
    mEventGroup->AddEvent(metricEventPtr);
    mEventGroup->AddEvent(spanEventPtr);
    EventsContainer eventContainer;
    mEventGroup->SwapEvents(eventContainer);
    APSARA_TEST_EQUAL_FATAL(3L, eventContainer.size());
    APSARA_TEST_EQUAL_FATAL(0L, mEventGroup->GetEvents().size());
}

void PipelineEventGroupUnittest::TestSetMetadata() {
    { // string copy, let kv out of scope
        mEventGroup->SetMetadata(std::string("key1"), std::string("value1"));
    }
    { // stringview copy, let kv out of scope
        std::string key("key2");
        std::string value("value2");
        mEventGroup->SetMetadata(StringView(key), StringView(value));
    }
    size_t beforeAlloc;
    { // StringBuffer nocopy
        StringBuffer key = mEventGroup->GetSourceBuffer()->CopyString(std::string("key3"));
        StringBuffer value = mEventGroup->GetSourceBuffer()->CopyString(std::string("value3"));
        beforeAlloc = mSourceBuffer->mAllocator.TotalAllocated();
        mEventGroup->SetMetadataNoCopy(key, value);
    }
    std::string key("key4");
    std::string value("value4");
    { // StringView nocopy
        mEventGroup->SetMetadataNoCopy(StringView(key), StringView(value));
    }
    size_t afterAlloc = mSourceBuffer->mAllocator.TotalAllocated();
    APSARA_TEST_EQUAL_FATAL(beforeAlloc, afterAlloc);
    std::vector<std::pair<std::string, std::string>> answers = {
        {"key1", "value1"},
        {"key2", "value2"},
        {"key3", "value3"},
        {"key4", "value4"},
    };
    for (const auto kv : answers) {
        APSARA_TEST_TRUE_FATAL(mEventGroup->HasMetadata(kv.first));
        APSARA_TEST_STREQ_FATAL(kv.second.c_str(), mEventGroup->GetMetadata(kv.first).data());
    }
}

void PipelineEventGroupUnittest::TestDelMetadata() {
    mEventGroup->SetMetadata(std::string("key1"), std::string("value1"));
    APSARA_TEST_TRUE_FATAL(mEventGroup->HasMetadata("key1"));
    mEventGroup->DelMetadata(std::string("key1"));
    APSARA_TEST_FALSE_FATAL(mEventGroup->HasMetadata("key1"));
}

void PipelineEventGroupUnittest::TestFromJsonToJson() {
    std::string inJson = R"({
        "events" :
        [
            {
                "contents" :
                {
                    "key1" : "value1",
                    "key2" : "value2"
                },
                "timestamp" : 12345678901,
                "timestampNanosecond" : 0,
                "type" : 1
            }
        ],
        "metadata" :
        {
            "log.file.path" : "/var/log/message"
        },
        "tags" :
        {
            "app_name" : "xxx"
        }
    })";
    APSARA_TEST_TRUE_FATAL(mEventGroup->FromJsonString(inJson));
    auto& events = mEventGroup->GetEvents();
    APSARA_TEST_EQUAL_FATAL(1L, events.size());
    auto& logEvent = events[0];
    APSARA_TEST_TRUE_FATAL(logEvent.Is<LogEvent>());

    APSARA_TEST_TRUE_FATAL(mEventGroup->HasMetadata("log.file.path"));
    APSARA_TEST_STREQ_FATAL("/var/log/message", mEventGroup->GetMetadata("log.file.path").data());

    APSARA_TEST_TRUE_FATAL(mEventGroup->HasTag("app_name"));
    APSARA_TEST_STREQ_FATAL("xxx", mEventGroup->GetTag("app_name").data());

    std::string outJson = mEventGroup->ToJsonString();
    APSARA_TEST_STREQ_FATAL(CompactJson(inJson).c_str(), CompactJson(outJson).c_str());
}

} // namespace logtail

UNIT_TEST_MAIN