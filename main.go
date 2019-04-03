package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/prometheus/util/promlint"
)

var metricPattern = regexp.MustCompile(`^([a-zA-Z_:]([a-zA-Z0-9_:])*): (.*)$`)
var logger log.Logger
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

func init() {
	logger = log.NewLogfmtLogger(os.Stderr)
	logger = level.NewFilter(logger, getLogLevel())
	logger = log.With(logger, "ts", log.DefaultTimestampUTC)

	prometheus.MustRegister(probeSuccessCount)
	prometheus.MustRegister(probeFailureCount)
	prometheus.MustRegister(probeDurationCount)
}

func getLogLevel() level.Option {
	logLevel := getEnv("COMMONSTATUS_EXPORTER_LOG_LEVEL", "INFO")
	switch strings.ToUpper(logLevel) {
	case "DEBUG":
		return level.AllowDebug()
	case "INFO":
		return level.AllowInfo()
	case "WARN", "WARNING":
		return level.AllowWarn()
	case "ERROR":
		return level.AllowError()
	default:
		return level.AllowInfo()
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	level.Warn(logger).Log("msg", "env variable doesn't set, using default value", "variable", key, "default_value", fallback)
	return fallback
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
	timeoutSeconds, err := strconv.Atoi(getEnv("COMMONSTATUS_CONNECTION_TIMEOUT", "8"))
	if err != nil {
		level.Warn(logger).Log("msg", "Wrong value of COMMONSTATUS_CONNECTION_TIMEOUT environment variable, using default value", "err", err)
		timeoutSeconds = 8
	}
	client := &http.Client{
		Timeout: time.Duration(timeoutSeconds) * time.Second,
	}
	resp, err := client.Get(c.hostURL)
	if err != nil {
		level.Info(logger).Log("msg", "failed to connect to the host", "err", err, "hostURL", c.hostURL)
		c.probeFailure(ch)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		level.Info(logger).Log("msg", "HTTP response status code is not 200", "code", resp.StatusCode, "hostURL", c.hostURL)
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
		level.Debug(logger).Log("msg", "received a new metric", "metric", metric, "host", c.hostURL)
		// TODO: move all this section to convert?
		if isValidMetric(metric) && len(metric) > 0 {
			name := metricPattern.FindStringSubmatch(metric)[1]
			value := metricPattern.FindStringSubmatch(metric)[3]
			floatValue, err := strconv.ParseFloat(value, 64)
			if err != nil {
				level.Debug(logger).Log("msg", "error converting to float64", "metric", metric, "err", err)
				failed++
				continue
			}
			desc := prometheus.NewDesc(name, "", nil, nil)
			ch <- prometheus.MustNewConstMetric(desc, prometheus.UntypedValue, floatValue)

			converted++
			level.Debug(logger).Log("msg", "successfully added metric to the registry", "metric", metric)
		} else {
			level.Debug(logger).Log("msg", "the metric is not valid, trying to convert it", "metric", metric)
			// fixAndAddMetric(metric)
			err := convertMetric(metric, ch)
			if err != nil {
				level.Debug(logger).Log("msg", "failed to convert it", "metric", metric, "err", err)
				failed++
				continue
			}

			converted++
			level.Debug(logger).Log("msg", "converted and added metric to the registry", "metric", metric)
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
		level.Info(logger).Log("msg", "error ocurred during reading the response body", "err", err)
		c.probeFailure(ch)
		return
	}

	probeDurationCount.Add(duration)
	probeSuccessCount.Inc()
	ch <- prometheus.MustNewConstMetric(up, prometheus.GaugeValue, 1)
	level.Info(logger).Log("msg", "probe succeeded", "host", c.hostURL, "duration", fmt.Sprintf("%.2f s.", duration), "coverted_metrics", converted, "failed_metrics", failed)
}

func probeHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	timeoutSeconds := 2.0
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds*float64(time.Second)))
	defer cancel()

	// select {
	// case <-time.After(1 * time.Second):
	// 	fmt.Println("overslept")
	// 	http.Error(w, "Request should contain only one parameter: 'target'. Timeout exceeded", http.StatusRequestTimeout)
	// case <-ctx.Done():
	// 	fmt.Println(ctx.Err()) // prints "context deadline exceeded"
	// }

	r = r.WithContext(ctx)

	query := r.URL.Query()
	if len(query) > 1 {
		level.Warn(logger).Log("msg", "More than one parameter found in the request URL", "URL", r.URL)
		http.Error(w, "Request should contain only one parameter: 'target'. Encode the URL if needed.", http.StatusBadRequest)
		probeFailureCount.Inc()
		probeDurationCount.Add(time.Since(start).Seconds())
		return
	}
	target := query.Get("target")
	if target == "" {
		http.Error(w, "Parameter 'target' is missing", http.StatusBadRequest)
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
	http.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		probeHandler(w, r)
	})
	http.Handle("/metrics", promhttp.Handler())

	port := getEnv("COMMONSTATUS_EXPORTER_PORT", "9259")
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		level.Error(logger).Log("msg", "failed to start the server", "err", err)
		os.Exit(1)
	}

}
