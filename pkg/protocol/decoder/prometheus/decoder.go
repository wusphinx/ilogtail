// Copyright 2021 iLogtail Authors
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

package prometheus

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/richardartoul/molecule"
	"github.com/richardartoul/molecule/src/codec"

	"github.com/alibaba/ilogtail/pkg/models"
	"github.com/alibaba/ilogtail/pkg/protocol"
	"github.com/alibaba/ilogtail/pkg/protocol/decoder/common"
)

const (
	metricNameKey = "__name__"
	labelsKey     = "__labels__"
	timeNanoKey   = "__time_nano__"
	valueKey      = "__value__"
	tagDB         = "__tag__:db"
	metaDBKey     = "db"
)

const (
	contentEncodingKey = "Content-Encoding"
	contentTypeKey     = "Content-Type"
	pbContentType      = "application/x-protobuf"
	snappyEncoding     = "snappy"
)

// field index of the proto message models
// ref: https://github.com/prometheus/prometheus/blob/cb045c0e4b94bbf3eee174d91b5ef2b8553948d5/prompb/types.proto#L123
const (
	promPbFieldIndexTimeSeries      = 1
	promPbFieldIndexLabels          = 1
	promPbFieldIndexSamples         = 2
	promPbFieldIndexLabelName       = 1
	promPbFieldIndexLabelValue      = 2
	promPbFieldIndexSampleValue     = 1
	promPbFieldIndexSampleTimestamp = 2
)

// Decoder impl
type Decoder struct {
	AllowUnsafeMode bool
}

func parseLabels(metric model.Metric) (metricName, labelsValue string) {
	labels := (model.LabelSet)(metric)

	lns := make(model.LabelPairs, 0, len(labels))
	for k, v := range labels {
		lns = append(lns, &model.LabelPair{
			Name:  k,
			Value: v,
		})
	}
	sort.Sort(lns)
	var builder strings.Builder
	labelCount := 0
	for _, label := range lns {
		if label.Name == model.MetricNameLabel {
			metricName = string(label.Value)
			continue
		}
		if labelCount != 0 {
			builder.WriteByte('|')
		}
		builder.WriteString(string(label.Name))
		builder.WriteString("#$#")
		builder.WriteString(string(label.Value))
		labelCount++
	}
	return metricName, builder.String()
}

// Decode impl
func (d *Decoder) Decode(data []byte, req *http.Request, tags map[string]string) (logs []*protocol.Log, err error) {
	if req.Header.Get(contentEncodingKey) == snappyEncoding &&
		strings.HasPrefix(req.Header.Get(contentTypeKey), pbContentType) {
		return d.decodeInRemoteWriteFormat(data, req)
	}
	return d.decodeInExpFmt(data, req)
}

func (d *Decoder) decodeInExpFmt(data []byte, _ *http.Request) (logs []*protocol.Log, err error) {
	decoder := expfmt.NewDecoder(bytes.NewReader(data), expfmt.FmtText)
	sampleDecoder := expfmt.SampleDecoder{
		Dec: decoder,
		Opts: &expfmt.DecodeOptions{
			Timestamp: model.Now(),
		},
	}

	for {
		s := &model.Vector{}
		if err = sampleDecoder.Decode(s); err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			break
		}
		for _, sample := range *s {
			metricName, labelsValue := parseLabels(sample.Metric)
			log := &protocol.Log{
				Contents: []*protocol.Log_Content{
					{
						Key:   metricNameKey,
						Value: metricName,
					},
					{
						Key:   labelsKey,
						Value: labelsValue,
					},
					{
						Key:   timeNanoKey,
						Value: strconv.FormatInt(sample.Timestamp.UnixNano(), 10),
					},
					{
						Key:   valueKey,
						Value: strconv.FormatFloat(float64(sample.Value), 'g', -1, 64),
					},
				},
			}
			protocol.SetLogTimeWithNano(log, uint32(sample.Timestamp.Unix()), uint32(sample.Timestamp.UnixNano()%1e9))
			logs = append(logs, log)
		}
	}

	return logs, err
}

func (d *Decoder) ParseRequest(res http.ResponseWriter, req *http.Request, maxBodySize int64) (data []byte, statusCode int, err error) {
	return common.CollectBody(res, req, maxBodySize)
}

func (d *Decoder) decodeInRemoteWriteFormat(data []byte, req *http.Request) (logs []*protocol.Log, err error) {
	var metrics prompb.WriteRequest
	err = proto.Unmarshal(data, &metrics)
	if err != nil {
		return nil, err
	}

	db := req.FormValue(metaDBKey)
	contentLen := 4
	if len(db) > 0 {
		contentLen++
	}

	for _, m := range metrics.Timeseries {
		metricName, labelsValue := d.parsePbLabels(m.Labels)
		for _, sample := range m.Samples {
			contents := make([]*protocol.Log_Content, 0, contentLen)
			contents = append(contents, &protocol.Log_Content{
				Key:   metricNameKey,
				Value: metricName,
			}, &protocol.Log_Content{
				Key:   labelsKey,
				Value: labelsValue,
			}, &protocol.Log_Content{
				Key:   timeNanoKey,
				Value: strconv.FormatInt(sample.Timestamp*1e6, 10),
			}, &protocol.Log_Content{
				Key:   valueKey,
				Value: strconv.FormatFloat(sample.Value, 'g', -1, 64),
			})
			if len(db) > 0 {
				contents = append(contents, &protocol.Log_Content{
					Key:   tagDB,
					Value: db,
				})
			}

			log := &protocol.Log{
				Contents: contents,
			}
			protocol.SetLogTimeWithNano(log, uint32(model.Time(sample.Timestamp).Unix()), uint32(model.Time(sample.Timestamp).UnixNano()%1e9))
			logs = append(logs, log)
		}
	}

	return logs, nil
}

func (d *Decoder) parsePbLabels(labels []prompb.Label) (metricName, labelsValue string) {
	var builder strings.Builder
	for _, label := range labels {
		if label.Name == model.MetricNameLabel {
			metricName = label.Value
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte('|')
		}
		builder.WriteString(label.Name)
		builder.WriteString("#$#")
		builder.WriteString(label.Value)
	}
	return metricName, builder.String()
}

func (d *Decoder) DecodeV2(data []byte, req *http.Request) (groups []*models.PipelineGroupEvents, err error) {
	if len(data) == 0 {
		return nil, nil
	}

	meta := models.NewMetadata()
	commonTags := models.NewTags()
	if db := req.FormValue(metaDBKey); len(db) > 0 {
		meta.Add(metaDBKey, db)
	}

	if req.Header.Get(contentEncodingKey) == snappyEncoding &&
		strings.HasPrefix(req.Header.Get(contentTypeKey), pbContentType) {
		var groupEvents *models.PipelineGroupEvents
		if d.AllowUnsafeMode {
			groupEvents, err = ParsePromPbToPipelineGroupEventsUnsafe(data, meta, commonTags)
		} else {
			groupEvents, err = ConvertPromRequestToPipelineGroupEvents(data, meta, commonTags)
		}
		if err != nil {
			return nil, err
		}
		return []*models.PipelineGroupEvents{groupEvents}, nil
	}

	return ConvertExpFmtDataToPipelineGroupEvents(data, meta, commonTags)
}

func ConvertExpFmtDataToPipelineGroupEvents(data []byte, metaInfo models.Metadata, commonTags models.Tags) (pg []*models.PipelineGroupEvents, err error) {
	decoder := expfmt.NewDecoder(bytes.NewReader(data), expfmt.FmtText)
	sampleDecoder := expfmt.SampleDecoder{
		Dec: decoder,
		Opts: &expfmt.DecodeOptions{
			Timestamp: model.Now(),
		},
	}

	for {
		s := model.Vector{}
		if err = sampleDecoder.Decode(&s); err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			break
		}
		groupEvents, err := ConvertVectorToPipelineGroupEvents(&s, metaInfo, commonTags)
		if err == nil {
			pg = append(pg, groupEvents)
		}
	}

	return
}

func ConvertVectorToPipelineGroupEvents(s *model.Vector, metaInfo models.Metadata, commonTags models.Tags) (*models.PipelineGroupEvents, error) {
	groupEvent := &models.PipelineGroupEvents{
		Group: models.NewGroup(metaInfo, commonTags),
	}

	for _, sample := range *s {
		metricName, tags := parseModelMetric(sample.Metric)
		metric := models.NewSingleValueMetric(
			metricName,
			models.MetricTypeGauge,
			tags,
			sample.Timestamp.UnixNano(),
			sample.Value,
		)
		groupEvent.Events = append(groupEvent.Events, metric)
	}

	return groupEvent, nil
}

func parseModelMetric(metric model.Metric) (string, models.Tags) {
	var metricName string
	tags := models.NewTags()
	labels := (model.LabelSet)(metric)

	for k, v := range labels {
		if k == model.MetricNameLabel {
			metricName = string(v)
			continue
		}
		tags.Add(string(k), string(v))
	}

	return metricName, tags
}

func ConvertPromRequestToPipelineGroupEvents(data []byte, metaInfo models.Metadata, commonTags models.Tags) (*models.PipelineGroupEvents, error) {
	var promRequest prompb.WriteRequest
	err := proto.Unmarshal(data, &promRequest)
	if err != nil {
		return nil, err
	}

	groupEvent := &models.PipelineGroupEvents{
		Group: models.NewGroup(metaInfo, commonTags),
	}

	for _, ts := range promRequest.Timeseries {
		var metricName string
		tags := models.NewTags()
		for _, label := range ts.Labels {
			if label.Name == metricNameKey {
				metricName = label.Value
				continue
			}
			tags.Add(label.Name, label.Value)
		}

		for _, dataPoint := range ts.Samples {
			metric := models.NewSingleValueMetric(
				metricName,
				models.MetricTypeGauge,
				tags,
				model.Time(dataPoint.GetTimestamp()).Time().UnixNano(),
				dataPoint.GetValue(),
			)
			groupEvent.Events = append(groupEvent.Events, metric)
		}
	}

	return groupEvent, nil
}

func ParsePromPbToPipelineGroupEventsUnsafe(data []byte, metaInfo models.Metadata, commonTags models.Tags) (*models.PipelineGroupEvents, error) {
	groupEvent := &models.PipelineGroupEvents{
		Group: models.NewGroup(metaInfo, commonTags),
	}

	buffer := codec.NewBuffer(data)
	err1 := molecule.MessageEach(buffer, func(fieldNum int32, v molecule.Value) (bool, error) {
		if fieldNum == promPbFieldIndexTimeSeries {
			serieBytes, err2 := v.AsBytesUnsafe()
			if err2 != nil {
				return false, err2
			}

			var metricName string
			metricTags := models.NewTags()

			serieBuffer := codec.NewBuffer(serieBytes)
			err2 = molecule.MessageEach(serieBuffer, func(fieldNum int32, v molecule.Value) (bool, error) {
				switch fieldNum {
				case promPbFieldIndexLabels: // Labels
					labelBytes, err3 := v.AsBytesUnsafe()
					if err3 != nil {
						return false, err3
					}

					var name, value string

					labelBuffer := codec.NewBuffer(labelBytes)
					err3 = molecule.MessageEach(labelBuffer, func(fieldNum int32, v molecule.Value) (bool, error) {
						var err4 error
						switch fieldNum {
						case promPbFieldIndexLabelName: // Name
							name, err4 = v.AsStringUnsafe()
							if err4 != nil {
								return false, err4
							}
						case promPbFieldIndexLabelValue: // Value
							value, err4 = v.AsStringUnsafe()
							if err4 != nil {
								return false, err4
							}
						}
						return true, nil
					})
					if err3 != nil {
						return false, err3
					}

					if name == metricNameKey {
						metricName = value
					} else {
						metricTags.Add(name, value)
					}
				case promPbFieldIndexSamples: // Samples
					sampleBytes, err3 := v.AsBytesUnsafe()
					if err3 != nil {
						return false, err3
					}

					var value float64
					var timestamp int64

					sampleBuffer := codec.NewBuffer(sampleBytes)
					err3 = molecule.MessageEach(sampleBuffer, func(fieldNum int32, v molecule.Value) (bool, error) {
						var err4 error
						switch fieldNum {
						case promPbFieldIndexSampleValue: // Value
							value, err4 = v.AsDouble()
							if err4 != nil {
								return false, err4
							}
						case promPbFieldIndexSampleTimestamp: // Timestamp
							timestamp, err4 = v.AsInt64()
							if err4 != nil {
								return false, err4
							}
						}
						return true, nil
					})
					if err3 != nil {
						return false, err3
					}

					if metricName == "" {
						return false, fmt.Errorf("fields in mix order")
					}

					metric := models.NewSingleValueMetric(
						metricName,
						models.MetricTypeGauge,
						metricTags,
						timestamp*1e6,
						value,
					)
					groupEvent.Events = append(groupEvent.Events, metric)
				}
				return true, nil
			})
			if err2 != nil {
				return false, err2
			}
		}
		return true, nil
	})
	if err1 != nil {
		return nil, err1
	}
	return groupEvent, nil
}
