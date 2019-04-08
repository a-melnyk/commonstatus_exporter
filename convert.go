package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type MetricType int

const (
	LoadAvg MetricType = iota
	StartupTime
	ReleaseTag
	RunningAverages
	Other
)

var (
	invalidChars    = regexp.MustCompile(`[^a-zA-Z0-9:_]`)
	loadAvg         = regexp.MustCompile(`^LoadAvg:\s+(?P<la1m>\d+(\.\d+)?) (?P<la5m>\d+(\.\d+)?) (?P<la15m>\d+(\.\d+)?)$`)
	numbericValue   = regexp.MustCompile(`^([0-9]+[0-9,.]*[0-9]*)$`)
	metricTemplate  = regexp.MustCompile(`^([a-zA-Z_:].*):\s+(.+)$`)
	startupTime     = regexp.MustCompile(`^StartupTime:\s+(.*)$`)
	releaseTag      = regexp.MustCompile(`^ReleaseTag:\s+(.*)$`)
	runningAverages = regexp.MustCompile(`^(.+):\s+count=([0-9]+[0-9,.]*) averageValue=([0-9]+[0-9,.]*) realMaxValue=([0-9]+[0-9,.]*) averageEventRate=[0-9]+[0-9,.]* maxEventRate=[0-9]+[0-9,.]* stdDeviation=([0-9]+[0-9,.]*) maxValue=[0-9]+[0-9,.]*$`)
)

func parseValue(value string) (float64, error) {
	if !(numbericValue.MatchString(value)) {
		return 0, fmt.Errorf("can't parse metric, invalid value: %s", value)
	}

	value = strings.Replace(value, ",", "", -1)
	return strconv.ParseFloat(value, 64)
}

func createPrometheusMetricWithLabels(name string, desc string, value float64, labels prometheus.Labels, metricType prometheus.ValueType) (prometheus.Metric, error) {
	name = invalidChars.ReplaceAllLiteralString(name, "_")
	promDesc := prometheus.NewDesc(name, desc, nil, labels)
	metric, err := prometheus.NewConstMetric(promDesc, metricType, value)
	if err != nil {
		return nil, err
	}
	return metric, nil
}

func createPrometheusMetric(name string, desc string, value float64, metricType prometheus.ValueType) (prometheus.Metric, error) {
	return createPrometheusMetricWithLabels(name, desc, value, nil, metricType)
}

func convertAndCreatePrometheusMetric(name string, desc string, value string, metricType prometheus.ValueType) (prometheus.Metric, error) {
	parsedValue, err := parseValue(value)
	if err != nil {
		return nil, err
	}
	return createPrometheusMetric(name, desc, parsedValue, metricType)
}

func convertLoadAvg(metric string, ch chan<- prometheus.Metric) error {
	if !(loadAvg.MatchString(metric)) {
		return fmt.Errorf("no LoadAvg metric found in: %s", metric)
	}

	matchResult := loadAvg.FindStringSubmatch(metric)

	la1Metric, err := convertAndCreatePrometheusMetric("load_avertage1", "1m load average.", matchResult[1], prometheus.GaugeValue)
	if err != nil {
		return err
	}

	la5Metric, err := convertAndCreatePrometheusMetric("load_avertage5", "5m load average.", matchResult[3], prometheus.GaugeValue)
	if err != nil {
		return err
	}

	la15Metric, err := convertAndCreatePrometheusMetric("load_avertage15", "15m load average.", matchResult[5], prometheus.GaugeValue)
	if err != nil {
		return err
	}

	ch <- la1Metric
	ch <- la5Metric
	ch <- la15Metric

	return nil
}

func convertStartupTime(metric string, ch chan<- prometheus.Metric) error {
	if !(startupTime.MatchString(metric)) {
		return fmt.Errorf("no metric with numberic value found in: %s", metric)
	}

	value := startupTime.FindStringSubmatch(metric)[1]

	parsedTime, err := time.Parse(time.UnixDate, value)
	if err != nil {
		return err
	}
	uptime := time.Since(parsedTime).Seconds()

	promMetric, err := createPrometheusMetric("app_uptime_seconds_total", "Time that an application is running", uptime, prometheus.CounterValue)
	if err != nil {
		return err
	}

	ch <- promMetric
	return nil
}

func createInfoMetric(metric string, ch chan<- prometheus.Metric) error {
	if !releaseTag.MatchString(metric) {
		return fmt.Errorf("the metric doesn't contain a ReleaseTag: %s", metric)
	}

	infoLabels := prometheus.Labels{
		"release_tag": releaseTag.FindStringSubmatch(metric)[1],
	}
	promMetric, err := createPrometheusMetricWithLabels("commonstatus_info", "CommonStatus information", float64(1), infoLabels, prometheus.GaugeValue)
	if err != nil {
		return err
	}

	ch <- promMetric

	return nil
}

func convertRunningAverages(metric string, ch chan<- prometheus.Metric) error {
	if !runningAverages.MatchString(metric) {
		return fmt.Errorf("the metric doesn't contain a RunningAverages: %s", metric)
	}

	/*
		TimeSearch:
		count=77 -> creating Prometheus metric TimeSearch_total
		averageValue=275 -> creating Prometheus metric TimeSearch_seconds_total: count*averageValue/1000
		realMaxValue=2,784 -> creating Prometheus metric TimeSearch_max_seconds: realMaxValue/1000
		averageEventRate=1.283 -> dropping, use Prometheus rate() instead
		maxEventRate=3 -> dropping, use Prometheus rate() instead + max_over_time()
		stdDeviation=409 -> creating Prometheus metric TimeSearch_stddev_seconds: stdDeviation/1000
		maxValue=684 (-)  -> dropping, use avg + stddev instead
	*/
	matchResult := runningAverages.FindStringSubmatch(metric)

	metricName := matchResult[1]
	count, err := parseValue(matchResult[2])
	if err != nil {
		return err
	}
	averageValue, err := parseValue(matchResult[3])
	if err != nil {
		return err
	}
	realMaxValue, err := parseValue(matchResult[4])
	if err != nil {
		return err
	}
	stdDeviation, err := parseValue(matchResult[5])
	if err != nil {
		return err
	}

	total, err := createPrometheusMetric(metricName+"_total", "Total number of "+metricName+" requests", count, prometheus.CounterValue)
	if err != nil {
		return err
	}

	secondsTotal, err := createPrometheusMetric(metricName+"_seconds_total", "Total duration of "+metricName+" requests", count*averageValue/1000, prometheus.CounterValue)
	if err != nil {
		return err
	}

	maxSeconds, err := createPrometheusMetric(metricName+"_max_seconds", "Maximal duration of "+metricName+" request", realMaxValue/1000, prometheus.GaugeValue)
	if err != nil {
		return err
	}

	stddevSeconds, err := createPrometheusMetric(metricName+"_stddev_seconds", "Standart deviation of "+metricName+" duration", stdDeviation/1000, prometheus.GaugeValue)
	if err != nil {
		return err
	}

	ch <- total
	ch <- secondsTotal
	ch <- maxSeconds
	ch <- stddevSeconds

	return nil
}

func defaultMetricsConverter(metric string, ch chan<- prometheus.Metric) error {
	matchResult := metricTemplate.FindStringSubmatch(metric)
	name := matchResult[1]
	value := matchResult[2]

	promMetric, err := convertAndCreatePrometheusMetric(name, "", value, prometheus.UntypedValue)
	if err != nil {
		return err
	}
	ch <- promMetric
	return nil
}

func metricType(metric string) MetricType {
	if releaseTag.MatchString(metric) {
		return ReleaseTag
	}

	if loadAvg.MatchString(metric) {
		return LoadAvg
	}

	if startupTime.MatchString(metric) {
		return StartupTime
	}

	if runningAverages.MatchString(metric) {
		return RunningAverages
	}

	return Other
}

func convertMetric(metric string, ch chan<- prometheus.Metric) error {
	if !metricTemplate.MatchString(metric) {
		return fmt.Errorf("the string doesn't contain a valid metric: %s", metric)
	}

	switch metricType(metric) {
	case ReleaseTag:
		return createInfoMetric(metric, ch)
	case LoadAvg:
		return convertLoadAvg(metric, ch)
	case StartupTime:
		return convertStartupTime(metric, ch)
	case RunningAverages:
		return convertRunningAverages(metric, ch)
	default:
		return defaultMetricsConverter(metric, ch)
	}
}
