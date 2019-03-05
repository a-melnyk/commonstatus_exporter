package main

import (
	"testing"
	"time"

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

	ch := make(chan prometheus.Metric, 3)
	defer close(ch)

	// WHEN
	err := patchLoadAvg(input, ch)

	// THEN
	assert.NoError(err)
	if err != nil {
		return
	}

	var results []prometheus.Metric
	for i := 0; i < len(wants); i++ {
		results = append(results, <-ch)
	}

	for i, want := range wants {
		result := results[i]
		wantDesc := want.Desc().String()
		resultDesc := result.Desc().String()
		assert.Equal(wantDesc, resultDesc, "descriptions are different! Wanted: %v, got: ", wantDesc, resultDesc)

		wantMetric := dto.Metric{}
		resultMetric := dto.Metric{}
		want.Write(&wantMetric)
		result.Write(&resultMetric)
		assert.Equal(wantMetric.String(), resultMetric.String(), "metrics are different! Wanted: %v, got: %v", wantMetric.String(), resultMetric.String())
	}
}

func TestPatchLoadAvg_invalidInput(t *testing.T) {
	assert := assert.New(t)

	// GIVEN
	invalidInput := []byte("LoadAvg: 1.94 3.44 5,07")

	ch := make(chan prometheus.Metric)
	defer close(ch)

	// WHEN
	err := patchLoadAvg(invalidInput, ch)

	// THEN
	assert.NotNilf(err, "patchLoadAvg should return error for invalid input")
}

func TestPatchNumberSeparators_ok(t *testing.T) {
	assert := assert.New(t)
	type testpair struct {
		metric []byte
		result float64
	}

	var tests = []testpair{
		{[]byte("MemoryUsed: 9,220,838,392"), 9220838392},
		{[]byte("MemoryUsed: 9.220.838.392"), 9220838392},
		{[]byte("MemoryUsed: 9220838392,01"), 9220838392.01},
		{[]byte("MemoryUsed: 9,220,838,392.01"), 9220838392.01},
		{[]byte("MemoryUsed: 9.220.838.392,01"), 9220838392.01},
		{[]byte("MemoryUsed: 4.997,14"), 4997.14},
		{[]byte("MemoryUsed: 4,997.14"), 4997.14},
	}

	for _, test := range tests {
		ch := make(chan prometheus.Metric, 1)
		defer close(ch)

		err := patchNumberSeparators(test.metric, ch)
		assert.NoError(err)
		if err != nil {
			return
		}

		result := <-ch
		resultDesc := result.Desc().String()

		wantDesc := prometheus.NewDesc("MemoryUsed", "", nil, nil)
		want := prometheus.MustNewConstMetric(wantDesc, prometheus.GaugeValue, test.result)
		assert.Equal(wantDesc.String(), resultDesc, "descriptions are different! Wanted: %v, got: %v, metric: %s", wantDesc, resultDesc, test.metric)

		wantMetric := dto.Metric{}
		resultMetric := dto.Metric{}
		want.Write(&wantMetric)
		result.Write(&resultMetric)
		assert.Equal(wantMetric.String(), resultMetric.String(), "metrics are different! Wanted: %v, got: %v, metric: %s", wantMetric.String(), resultMetric.String(), test.metric)
	}
}

func TestPatchStartupTime_ok(t *testing.T) {
	assert := assert.New(t)
	type testpair struct {
		metric    []byte
		timestamp int64
	}

	var tests = []testpair{
		{[]byte("StartupTime: Mon Jan 28 14:24:03 CET 2019"), 1548681843},
		{[]byte("StartupTime: Tue Jan 01 14:24:00 CET 2019"), 1546349040},
		{[]byte("StartupTime: Tue Jan 01 14:24:00 GMT 2019"), 1546352640},
	}

	for _, test := range tests {
		ch := make(chan prometheus.Metric, 1)
		defer close(ch)

		err := patchStartupTime(test.metric, ch)
		assert.NoError(err)
		if err != nil {
			return
		}

		result := <-ch
		resultDesc := result.Desc().String()

		wantDesc := prometheus.NewDesc("app_uptime_seconds_total", "Time that an application is running", nil, nil)
		uptime := time.Since(time.Unix(test.timestamp, 0)).Seconds()
		want := prometheus.MustNewConstMetric(wantDesc, prometheus.CounterValue, uptime)
		assert.Equal(wantDesc.String(), resultDesc, "descriptions are different! Wanted: %v, got: %v, metric: %s", wantDesc, resultDesc, test.metric)

		wantMetric := dto.Metric{}
		resultMetric := dto.Metric{}
		want.Write(&wantMetric)
		result.Write(&resultMetric)
		resultValue := *(resultMetric.GetCounter().Value)
		assert.InDelta(uptime, resultValue, 1, "metrics are different! Wanted: %v, got: %v, metric: %s", uptime, resultValue, test.metric)
	}
}
