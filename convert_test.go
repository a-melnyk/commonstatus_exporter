package main

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func TestPatchLoadAvg_ok(t *testing.T) {
	assert := assert.New(t)

	// GIVEN
	input := []byte("LoadAvg: 1.94 3.44 5.07")

	la1Desc := prometheus.NewDesc("load_avertage1", "1m load average.", nil, nil)
	la1Metric := prometheus.MustNewConstMetric(la1Desc, prometheus.GaugeValue, float64(1.94))

	la5Desc := prometheus.NewDesc("load_avertage5", "5m load average.", nil, nil)
	la5Metric := prometheus.MustNewConstMetric(la5Desc, prometheus.GaugeValue, float64(3.44))

	la15Desc := prometheus.NewDesc("load_avertage15", "15m load average.", nil, nil)
	la15Metric := prometheus.MustNewConstMetric(la15Desc, prometheus.GaugeValue, float64(5.07))

	wants := []prometheus.Metric{la1Metric, la5Metric, la15Metric}

	ch := make(chan prometheus.Metric)
	defer close(ch)

	// WHEN
	go patchLoadAvg(input, ch)

	// THEN
	var results []prometheus.Metric
	for i := 0; i < len(wants); i++ {
		results = append(results, <-ch)
	}

	for i, want := range wants {
		result := results[i]
		wantDesc := want.Desc().String()
		resultDesc := result.Desc().String()
		assert.Equal(wantDesc, resultDesc, "Descriptions are different! Wanted: %v, got: ", wantDesc, resultDesc)

		wantMetric := dto.Metric{}
		resultMetric := dto.Metric{}
		want.Write(&wantMetric)
		result.Write(&resultMetric)
		assert.Equal(wantMetric.String(), resultMetric.String(), "Metrics are different! Wanted: %v, got: %v", wantMetric.String(), resultMetric.String())
	}
}

func TestPatchLoadAvg_invalidInput(t *testing.T) {
	assert := assert.New(t)

	// GIVEN
	invalidInput := []byte("LoadAvg: 1.94 3.44 5,07")

	ch := make(chan prometheus.Metric)
	defer close(ch)

	// WHEN
	result := patchLoadAvg(invalidInput, ch)

	// THEN
	assert.NotNil(result, "patchLoadAvg must return error for invalid input")
}
