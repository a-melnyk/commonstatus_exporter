package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/util/promlint"
)

func isValidMetric(metric []byte) bool {
	l := promlint.New(bytes.NewReader(metric))

	if _, err := l.Lint(); err != nil {
		return false
	}

	return true
}

func createPrometheusMetric(name string, desc string, value string, metricType prometheus.ValueType) (prometheus.Metric, error) {
	promDesc := prometheus.NewDesc(name, desc, nil, nil)

	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, fmt.Errorf("can't parse: %v to float", value)
	}

	// TODO: fix metric name - replace invalid symbols with underscores

	metric := prometheus.MustNewConstMetric(promDesc, metricType, floatValue)
	return metric, nil
}

func patchLoadAvg(metric []byte, ch chan<- prometheus.Metric) error {
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

func patchNumberSeparators(metric []byte, ch chan<- prometheus.Metric) error {
	// https://docs.oracle.com/cd/E19455-01/806-0169/overview-9/index.html
	re := regexp.MustCompile(`^([a-zA-Z_:]([a-zA-Z0-9_:])*): ([0-9]([0-9,.])+[0-9])$`)

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

func patchStartupTime(metric []byte, ch chan<- prometheus.Metric) error {
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

func patchReleaseTag(metric []byte, ch chan<- prometheus.Metric) error {
	return nil
}

func fixAndAddMetric(metric []byte) ([]byte, error) {
	// r := regexp.MustCompile("^[a-zA-Z_:]([a-zA-Z0-9_:])*$")
	// patchLoadAvg(metric)
	// patchNumberSeparators(metric,)
	// patchStartupTime(metric)
	// patchReleaseTag(metric)
	return metric, nil
}

func processMethodRunTimeMetric(metric []byte) {
	// process MethodRunTime_ metric and add prom metrics to the registry
	if match, err := regexp.Match("^MethodRunTime_", metric); match && err == nil {
		// continue
	}
}

func main() { //convert()
	response, err := http.Get("http://localhost:8080/status?detail=all")
	if err != nil {
		fmt.Printf("http.get Error ocurred: %s", err)
		// ch <- prometheus.MustNewConstMetric(up, prometheus.GaugeValue, 0)
		return
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Printf("ReadAll Error ocurred: %s", err)
		// ch <- prometheus.MustNewConstMetric(up, prometheus.GaugeValue, 0)
		return
	}

	for i, line := range bytes.Split(body, []byte{'\n'}) {
		// fmt.Printf("Line #%d is: %s\n", i, line)

		metric := append(line, '\n')
		// metric := line

		// set instance and application_name  labels for all metrics
		// set commonstatus_info metric and label wil application versions
		fmt.Printf("Metric #%v is %s, byte: %v, len: %v", i, metric, metric, len(metric))

		if isValidMetric(metric) {
			// adding metric to the registry
			fmt.Println("Added metric to the registry!")
		} else {
			fmt.Println("The metric is not valid, trying to fix it")
			fixAndAddMetric(metric)
		}
	}
}
