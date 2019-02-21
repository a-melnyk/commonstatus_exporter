package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"

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

func patchLoadAvg(metric []byte, ch chan<- prometheus.Metric) error {
	re := regexp.MustCompile(`^LoadAvg: (?P<la1m>\d+(\.\d+)?) (?P<la5m>\d+(\.\d+)?) (?P<la15m>\d+(\.\d+)?)$`)

	if !(re.Match(metric)) {
		return fmt.Errorf("no LoadAvg metric found in: %s", metric)
	}

	matchResult := re.FindSubmatch(metric)

	la1Value, err := strconv.ParseFloat(string(matchResult[1]), 64)
	if err != nil {
		return fmt.Errorf("can't parse: %v to float", matchResult[1])
	}
	la5Value, err := strconv.ParseFloat(string(matchResult[3]), 64)
	if err != nil {
		return fmt.Errorf("can't parse: %v to float", matchResult[3])
	}
	la15Value, err := strconv.ParseFloat(string(matchResult[5]), 64)
	if err != nil {
		return fmt.Errorf("can't parse: %v to float", matchResult[5])
	}

	la1Desc := prometheus.NewDesc("load_avertage1", "1m load average.", nil, nil)
	la1Metric := prometheus.MustNewConstMetric(la1Desc, prometheus.GaugeValue, la1Value)

	la5Desc := prometheus.NewDesc("load_avertage5", "5m load average.", nil, nil)
	la5Metric := prometheus.MustNewConstMetric(la5Desc, prometheus.GaugeValue, la5Value)

	la15Desc := prometheus.NewDesc("load_avertage15", "15m load average.", nil, nil)
	la15Metric := prometheus.MustNewConstMetric(la15Desc, prometheus.GaugeValue, la15Value)

	ch <- la1Metric
	ch <- la5Metric
	ch <- la15Metric

	return nil
}

func patchNumberSeparators(metric []byte) []byte {
	// https://docs.oracle.com/cd/E19455-01/806-0169/overview-9/index.html
	return metric
}

func patchStartupTime(metric []byte) []byte {
	return metric
}

func patchReleaseTag(metric []byte) []byte {
	return metric
}

func fixAndAddMetric(metric []byte) ([]byte, error) {
	// r := regexp.MustCompile("^[a-zA-Z_:]([a-zA-Z0-9_:])*$")
	// patchLoadAvg(metric)
	patchNumberSeparators(metric)
	patchStartupTime(metric)
	patchReleaseTag(metric)
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
