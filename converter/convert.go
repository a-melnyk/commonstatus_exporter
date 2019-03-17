package main

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func createPrometheusMetric(name string, desc string, value string, metricType prometheus.ValueType) (prometheus.Metric, error) {
	invalidChars := regexp.MustCompile("[^a-zA-Z0-9:_]")
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
	re := regexp.MustCompile(`^LoadAvg: (?P<la1m>\d+(\.\d+)?) (?P<la5m>\d+(\.\d+)?) (?P<la15m>\d+(\.\d+)?)$`)

	if !(re.Match(metric)) {
		return fmt.Errorf("no LoadAvg metric found in: %s", metric)
	}

	matchResult := re.FindSubmatch(metric)

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
	re := regexp.MustCompile(`^([a-zA-Z_:]([a-zA-Z0-9_:])*): ([0-9]+([0-9,.])*[0-9]*)$`)

	if !(re.Match(metric)) {
		return fmt.Errorf("no metric with numberic value found in: %s", metric)
	}

	name := re.FindSubmatch(metric)[1]
	value := re.FindSubmatch(metric)[3]

	re = regexp.MustCompile(`^[0-9]([0-9,])+[0-9]$`)
	if re.Match(value) {
		value = bytes.Replace(value, []byte(","), []byte("."), -1)
	}

	re = regexp.MustCompile(`^[0-9]+(\.[0-9]+){2,}$`)
	if re.Match(value) {
		resultValue := bytes.Replace(value, []byte("."), []byte(""), -1)
		promMetric, err := createPrometheusMetric(string(name), "", string(resultValue), prometheus.GaugeValue)
		if err != nil {
			return err
		}
		ch <- promMetric
		return nil
	}

	re = regexp.MustCompile(`^[0-9]+(.[0-9]+)+,[0-9]+$`)
	if re.Match(value) {
		value = bytes.Replace(value, []byte("."), []byte(""), -1)
		value = bytes.Replace(value, []byte(","), []byte("."), -1)
		promMetric, err := createPrometheusMetric(string(name), "", string(value), prometheus.GaugeValue)
		if err != nil {
			return err
		}
		ch <- promMetric
		return nil
	}

	re = regexp.MustCompile(`^[0-9]+(,[0-9]+)+.[0-9]+$`)
	if re.Match(value) {
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
	re := regexp.MustCompile(`^StartupTime: (.*)$`)

	if !(re.Match(metric)) {
		return fmt.Errorf("no metric with numberic value found in: %s", metric)
	}

	value := re.FindSubmatch(metric)[1]

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
	re := regexp.MustCompile(`^ReleaseTag: (.*)$`)

	if !re.Match(metric) {
		return fmt.Errorf("the metric doesn't contain a ReleaseTag: %s", metric)
	}

	releaseTag := string(re.FindSubmatch(metric)[1])
	(*infoLabels)["release_tag"] = releaseTag

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
	re := regexp.MustCompile(`^([a-zA-Z_:]([a-zA-Z0-9_:])*): .*$`)
	if !re.Match(metric) {
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
