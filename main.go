package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/prometheus/util/promlint"
)

var up = prometheus.NewDesc(
	"application_host_up",
	"Was talking to the application successful.",
	nil, nil,
)

type CommonStatusExporter struct {
	hostURL string
}

func isValidMetric(metric []byte) bool {
	l := promlint.New(bytes.NewReader(metric))

	if _, err := l.Lint(); err != nil {
		return false
	}

	return true
}

// Implements prometheus.Collector.
func (c CommonStatusExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
}

// Implements prometheus.Collector.
func (c CommonStatusExporter) Collect(ch chan<- prometheus.Metric) {
	// response, err := http.Get("http://localhost:8080/status?detail=all")
	// if (err != nil) || (response.StatusCode != 200) {
	// 	ch <- prometheus.MustNewConstMetric(up, prometheus.GaugeValue, 0)
	// 	return
	// }

	// body, err := ioutil.ReadAll(response.Body)
	body, err := ioutil.ReadFile("valid_metrics.txt")
	// response.Body.Close()

	if err != nil {
		fmt.Printf("ReadAll Error ocurred: %s", err)
		ch <- prometheus.MustNewConstMetric(up, prometheus.GaugeValue, 0)
		return
	}
	ch <- prometheus.MustNewConstMetric(up, prometheus.GaugeValue, 1)

	for _, line := range bytes.Split(body, []byte{'\n'}) {
		if isValidMetric(line) {
			// adding metric to the registry
			fmt.Println("Added metric to the registry!")
		} else {
			fmt.Println("The metric is not valid, trying to convert it")
			// fixAndAddMetric(metric)
			err := convertMetric(line, ch)
			if err != nil {
				fmt.Printf("Failed to convert the metric: %s, error: %v\n", line, err)
			}
		}
	}
}

func main() {
	c := CommonStatusExporter{}
	prometheus.MustRegister(c)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":8000", nil))
}
