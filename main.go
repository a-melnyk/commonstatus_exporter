package main

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/prometheus/util/promlint"
)

var metricPattern = regexp.MustCompile(`^([a-zA-Z_:]([a-zA-Z0-9_:])*): (.*)$`)
var (
	up = prometheus.NewDesc(
		"up",
		"Was talking to application successfull",
		nil, nil,
	)
	probeSuccessCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "probe_success_total",
		Help: "Displays count of successfull probes",
	})
	probeFailureCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "probe_failure_total",
		Help: "Displays count of failed probes",
	})
	probeDurationCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "probe_seconds_total",
		Help: "Displays total duration of all probes",
	})
)

type CommonStatusExporter struct {
	hostURL   string
	startTime time.Time
}

func isValidMetric(metric string) bool {
	l := promlint.New(strings.NewReader(metric + "\n"))

	if _, err := l.Lint(); err != nil {
		return false
	}

	return true
}

func (c CommonStatusExporter) probeFailure(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(up, prometheus.GaugeValue, 0)
	probeFailureCount.Inc()
	probeDurationCount.Add(time.Since(c.startTime).Seconds())
}

// Implements prometheus.Collector.
func (c CommonStatusExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
}

// Implements prometheus.Collector.
func (c CommonStatusExporter) Collect(ch chan<- prometheus.Metric) {
	// TODO: check if we have configured timeout
	resp, err := http.Get(c.hostURL)
	if err != nil {
		log.Printf("Error during http get request, err: %v", err)
		c.probeFailure(ch)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("HTTP response status code is not 200: %v", resp.StatusCode)
		c.probeFailure(ch)
		return
	}

	// Wrap response body into buffered scanner. Set split function to scanLines
	s := bufio.NewScanner(resp.Body)
	s.Split(bufio.ScanLines)

	// iterate over lines
	var converted, failed float64
	for s.Scan() {
		metric := s.Text()
		log.Printf("Received metric: %v", metric)
		// TODO: move all this section to convert?
		if isValidMetric(metric) && len(metric) > 0 {
			name := metricPattern.FindStringSubmatch(metric)[1]
			value := metricPattern.FindStringSubmatch(metric)[3]
			floatValue, err := strconv.ParseFloat(value, 64)
			if err != nil {
				log.Printf("Error converting to float64, err: %v", err)
				failed++
				continue
			}
			desc := prometheus.NewDesc(name, "", nil, nil)
			ch <- prometheus.MustNewConstMetric(desc, prometheus.UntypedValue, floatValue)

			converted++
			log.Println("Added metric to the registry!")
		} else {
			log.Println("The metric is not valid, trying to convert it")

			// fixAndAddMetric(metric)
			err := convertMetric(metric, ch)
			if err != nil {
				log.Printf("Failed to convert metric, error: %v\n", err)
				failed++
				continue
			}

			converted++
			log.Println("Converted and added metric to the registry!")
		}
	}

	convertedMetricsGauge := prometheus.NewDesc("converted_metrics", "The number of CommonStatus metrics converted to prometheus metrics", nil, nil)
	ch <- prometheus.MustNewConstMetric(convertedMetricsGauge, prometheus.GaugeValue, converted)
	failedMetricsGauge := prometheus.NewDesc("failed_metrics", "The number of CommonStatus metrics failed to convert to prometheus metrics", nil, nil)
	ch <- prometheus.MustNewConstMetric(failedMetricsGauge, prometheus.GaugeValue, failed)

	duration := time.Since(c.startTime).Seconds()
	probeDurationGauge := prometheus.NewDesc("probe_duration_seconds", "Duration of the probe in seconds", nil, nil)
	ch <- prometheus.MustNewConstMetric(probeDurationGauge, prometheus.GaugeValue, duration)

	// check if errors ocurred during reading - e.g dropped connection or etc.
	if err := s.Err(); err != nil {
		log.Printf("error ocurred during the response body reading, err: %v", err)
		c.probeFailure(ch)
		return
	}

	probeDurationCount.Add(duration)
	probeSuccessCount.Inc()
	ch <- prometheus.MustNewConstMetric(up, prometheus.GaugeValue, 1)
	log.Printf("probe succeeded, duration: %v", duration)
}

func probeHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	timeoutSeconds := 10.0
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds*float64(time.Second)))
	defer cancel()
	r = r.WithContext(ctx)

	// TODO: treat all query string as target with all params
	target := r.URL.Query().Get("target")
	if target == "" {
		http.Error(w, "Target parameter is missing", http.StatusBadRequest)
		probeFailureCount.Inc()
		probeDurationCount.Add(time.Since(start).Seconds())
		return
	}

	registry := prometheus.NewRegistry()
	c := CommonStatusExporter{
		hostURL:   target,
		startTime: start,
	}
	registry.MustRegister(c)
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	h.ServeHTTP(w, r)
}

func main() {
	// TODO: log levels
	prometheus.MustRegister(probeSuccessCount)
	prometheus.MustRegister(probeFailureCount)
	prometheus.MustRegister(probeDurationCount)

	http.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		probeHandler(w, r)
	})
	http.Handle("/metrics", promhttp.Handler())

	// TODO: config option + file to choose port on which the exporter runs
	log.Fatal(http.ListenAndServe(":8080", nil))
}
