package main

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	invalidChars         = regexp.MustCompile(`[^a-zA-Z0-9:_]`)
	loadAvg              = regexp.MustCompile(`^LoadAvg: (?P<la1m>\d+(\.\d+)?) (?P<la5m>\d+(\.\d+)?) (?P<la15m>\d+(\.\d+)?)$`)
	numbericValue        = regexp.MustCompile(`^([a-zA-Z_:]([a-zA-Z0-9_:])*): ([0-9]+([0-9,.])*[0-9]*)$`)
	commaSeparator       = regexp.MustCompile(`^[0-9]([0-9,])+[0-9]$`)
	pointSeparator       = regexp.MustCompile(`^[0-9]+(\.[0-9]+){2,}$`)
	pointCommaSeparators = regexp.MustCompile(`^[0-9]+(.[0-9]+)+,[0-9]+$`)
	commaPointSeparators = regexp.MustCompile(`^[0-9]+(,[0-9]+)+.[0-9]+$`)
	validMetric          = regexp.MustCompile(`^([a-zA-Z_:]([a-zA-Z0-9_:])*): .*$`)
	startupTime          = regexp.MustCompile(`^StartupTime: (.*)$`)
	releaseTag           = regexp.MustCompile(`^ReleaseTag: (.*)$`)
)

func createPrometheusMetric(name string, desc string, value string, metricType prometheus.ValueType) (prometheus.Metric, error) {
	name = invalidChars.ReplaceAllLiteralString(name, "_")

	promDesc := prometheus.NewDesc(name, desc, nil, nil)

	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, fmt.Errorf("can't parse: %v to float", value)
	}

	metric := prometheus.MustNewConstMetric(promDesc, metricType, floatValue)
	return metric, nil
}

func convertLoadAvg(metric []byte, ch chan<- prometheus.Metric) error {
	if !(loadAvg.Match(metric)) {
		return fmt.Errorf("no LoadAvg metric found in: %s", metric)
	}

	matchResult := loadAvg.FindSubmatch(metric)

	la1Metric, err := createPrometheusMetric("load_avertage1", "1m load average.", string(matchResult[1]), prometheus.GaugeValue)
	if err != nil {
		return err
	}

	la5Metric, err := createPrometheusMetric("load_avertage5", "5m load average.", string(matchResult[3]), prometheus.GaugeValue)
	if err != nil {
		return err
	}

	la15Metric, err := createPrometheusMetric("load_avertage15", "15m load average.", string(matchResult[5]), prometheus.GaugeValue)
	if err != nil {
		return err
	}

	ch <- la1Metric
	ch <- la5Metric
	ch <- la15Metric

	return nil
}

func convertNumberSeparators(metric []byte, ch chan<- prometheus.Metric) error {
	if !(numbericValue.Match(metric)) {
		return fmt.Errorf("no metric with numberic value found in: %s", metric)
	}

	name := numbericValue.FindSubmatch(metric)[1]
	value := numbericValue.FindSubmatch(metric)[3]

	if commaSeparator.Match(value) {
		value = bytes.Replace(value, []byte(","), []byte("."), -1)
	}

	if pointSeparator.Match(value) {
		resultValue := bytes.Replace(value, []byte("."), []byte(""), -1)
		promMetric, err := createPrometheusMetric(string(name), "", string(resultValue), prometheus.GaugeValue)
		if err != nil {
			return err
		}
		ch <- promMetric
		return nil
	}

	if pointCommaSeparators.Match(value) {
		value = bytes.Replace(value, []byte("."), []byte(""), -1)
		value = bytes.Replace(value, []byte(","), []byte("."), -1)
		promMetric, err := createPrometheusMetric(string(name), "", string(value), prometheus.GaugeValue)
		if err != nil {
			return err
		}
		ch <- promMetric
		return nil
	}

	if commaPointSeparators.Match(value) {
		value = bytes.Replace(value, []byte(","), []byte(""), -1)
		promMetric, err := createPrometheusMetric(string(name), "", string(value), prometheus.GaugeValue)
		if err != nil {
			return err
		}
		ch <- promMetric
		return nil
	}

	promMetric, err := createPrometheusMetric(string(name), "", string(value), prometheus.GaugeValue)
	if err != nil {
		return err
	}
	ch <- promMetric
	return nil
}

func convertStartupTime(metric []byte, ch chan<- prometheus.Metric) error {
	if !(startupTime.Match(metric)) {
		return fmt.Errorf("no metric with numberic value found in: %s", metric)
	}

	value := startupTime.FindSubmatch(metric)[1]

	parsedTime, err := time.Parse(time.UnixDate, string(value))
	if err != nil {
		return err
	}
	uptime := time.Since(parsedTime).Seconds()

	promDesc := prometheus.NewDesc("app_uptime_seconds_total", "Time that an application is running", nil, nil)
	promMetric := prometheus.MustNewConstMetric(promDesc, prometheus.CounterValue, uptime)

	ch <- promMetric
	return nil
}

func parseReleaseTag(metric []byte, infoLabels *prometheus.Labels) error {
	if !releaseTag.Match(metric) {
		return fmt.Errorf("the metric doesn't contain a ReleaseTag: %s", metric)
	}

	releaseTagValue := string(releaseTag.FindSubmatch(metric)[1])
	(*infoLabels)["release_tag"] = releaseTagValue

	return nil
}

func createInfoMetric(infoLabels *prometheus.Labels, ch chan<- prometheus.Metric) {
	promDesc := prometheus.NewDesc("commonstatus_info", "commonstatus information", nil, *infoLabels)
	promMetric := prometheus.MustNewConstMetric(promDesc, prometheus.GaugeValue, float64(1))

	ch <- promMetric
}

func convertMethodRunTime(metric []byte) {
	// TODO: create function
	// process MethodRunTime_ metric and add prom metrics to the registry
	if match, err := regexp.Match("^MethodRunTime_", metric); match && err == nil {
		// continue
	}
}

// convertMetric converts []byte line into a prometheusMetric
func convertMetric(metric []byte, ch chan<- prometheus.Metric) error {
	if !validMetric.Match(metric) {
		return fmt.Errorf("the string doesn't contain a valid metric: %s", metric)
	}

	infoLabels := make(prometheus.Labels)
	if err := parseReleaseTag(metric, &infoLabels); err == nil {
		createInfoMetric(&infoLabels, ch)
		return nil
	}

	if err := convertLoadAvg(metric, ch); err == nil {
		return nil
	}

	if err := convertNumberSeparators(metric, ch); err == nil {
		return nil
	}

	if err := convertStartupTime(metric, ch); err == nil {
		return nil
	}

	return fmt.Errorf("Can't convert metric: %s. No suitable conversion function found", metric)
}
