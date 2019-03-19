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
	Other
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

func convertLoadAvg(metric string, ch chan<- prometheus.Metric) error {
	if !(loadAvg.MatchString(metric)) {
		return fmt.Errorf("no LoadAvg metric found in: %s", metric)
	}

	matchResult := loadAvg.FindStringSubmatch(metric)

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

func convertNumberSeparators(metric string, ch chan<- prometheus.Metric) error {
	if !(numbericValue.MatchString(metric)) {
		return fmt.Errorf("no metric with numberic value found in: %s", metric)
	}

	name := numbericValue.FindStringSubmatch(metric)[1]
	value := numbericValue.FindStringSubmatch(metric)[3]

	if commaSeparator.MatchString(value) {
		value = strings.Replace(value, ",", ".", -1)
	}

	if pointSeparator.MatchString(value) {
		resultValue := strings.Replace(value, ".", "", -1)
		promMetric, err := createPrometheusMetric(string(name), "", string(resultValue), prometheus.GaugeValue)
		if err != nil {
			return err
		}
		ch <- promMetric
		return nil
	}

	if pointCommaSeparators.MatchString(value) {
		value = strings.Replace(value, ".", "", -1)
		value = strings.Replace(value, ",", ".", -1)
		promMetric, err := createPrometheusMetric(string(name), "", string(value), prometheus.GaugeValue)
		if err != nil {
			return err
		}
		ch <- promMetric
		return nil
	}

	if commaPointSeparators.MatchString(value) {
		value = strings.Replace(value, ",", "", -1)
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

func convertStartupTime(metric string, ch chan<- prometheus.Metric) error {
	if !(startupTime.MatchString(metric)) {
		return fmt.Errorf("no metric with numberic value found in: %s", metric)
	}

	value := startupTime.FindStringSubmatch(metric)[1]

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

func parseReleaseTag(metric string, infoLabels *prometheus.Labels) error {
	if !releaseTag.MatchString(metric) {
		return fmt.Errorf("the metric doesn't contain a ReleaseTag: %s", metric)
	}

	releaseTagValue := string(releaseTag.FindStringSubmatch(metric)[1])
	(*infoLabels)["release_tag"] = releaseTagValue

	return nil
}

func createInfoMetric(infoLabels *prometheus.Labels, ch chan<- prometheus.Metric) {
	promDesc := prometheus.NewDesc("commonstatus_info", "commonstatus information", nil, *infoLabels)
	promMetric := prometheus.MustNewConstMetric(promDesc, prometheus.GaugeValue, float64(1))

	ch <- promMetric
}

func convertMethodRunTime(metric string) {
	// TODO: create function
	// process MethodRunTime_ metric and add prom metrics to the registry
	if match, err := regexp.MatchString("^MethodRunTime_", metric); match && err == nil {
		// continue
	}
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

	return Other
}

func convertMetric(metric string, ch chan<- prometheus.Metric) error {
	if !validMetric.MatchString(metric) {
		return fmt.Errorf("the string doesn't contain a valid metric: %s", metric)
	}

	switch metricType(metric) {
	case ReleaseTag:
		infoLabels := make(prometheus.Labels)
		if err := parseReleaseTag(metric, &infoLabels); err != nil {
			return err
		}
		createInfoMetric(&infoLabels, ch)
	case LoadAvg:
		return convertLoadAvg(metric, ch)
	case StartupTime:
		return convertStartupTime(metric, ch)
	default:
		return convertNumberSeparators(metric, ch)
	}

	return nil
}
