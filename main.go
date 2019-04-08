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

var metricPattern = regexp.MustCompile(`^([a-zA-Z_:].*):\s+(.+)$`)
var logger log.Logger
var timeoutSeconds float64
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
	hostURL        string
	metricsScanner *bufio.Scanner
	startTime      time.Time
}

func init() {
	logger = log.NewLogfmtLogger(os.Stderr)
	logger = level.NewFilter(logger, getLogLevel())
	logger = log.With(logger, "ts", log.DefaultTimestampUTC)

	prometheus.MustRegister(probeSuccessCount)
	prometheus.MustRegister(probeFailureCount)
	prometheus.MustRegister(probeDurationCount)

	var err error
	timeoutSeconds, err = strconv.ParseFloat(getEnv("CS_CONNECTION_TIMEOUT", "8.0"), 64)
	if err != nil {
		level.Error(logger).Log("msg", "Wrong value of CS_CONNECTION_TIMEOUT environment variable, using default value", "err", err)
		os.Exit(1)
	}
}

func getLogLevel() level.Option {
	logLevel := getEnv("CS_LOG_LEVEL", "INFO")
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
		level.Info(logger).Log("msg", "successfully processed env variable", "variable", key, "value", value)
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

// Implements prometheus.Collector.
func (c CommonStatusExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
}

// Implements prometheus.Collector.
func (c CommonStatusExporter) Collect(ch chan<- prometheus.Metric) {

	// Set split function to scanLines
	s := c.metricsScanner
	s.Split(bufio.ScanLines)

	// iterate over lines
	var converted, failed float64
	for s.Scan() {
		metric := s.Text()
		level.Debug(logger).Log("msg", "received a new metric", "metric", metric, "host", c.hostURL)
		if isValidMetric(metric) && len(metric) > 0 {
			name := metricPattern.FindStringSubmatch(metric)[1]
			value := metricPattern.FindStringSubmatch(metric)[2]
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
			err := convertMetric(metric, ch)
			if err != nil {
				level.Debug(logger).Log("msg", "failed to convert metric", "metric", metric, "err", err)
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

	probeDurationGauge := prometheus.NewDesc("probe_duration_seconds", "Duration of the probe in seconds", nil, nil)
	ch <- prometheus.MustNewConstMetric(probeDurationGauge, prometheus.GaugeValue, time.Since(c.startTime).Seconds())

	// check if errors ocurred during reading - e.g dropped connection or etc.
	if err := s.Err(); err != nil {
		level.Warn(logger).Log("msg", "error ocurred during reading the response body", "err", err)
		probeFailureCount.Inc()
		ch <- prometheus.MustNewConstMetric(up, prometheus.GaugeValue, 0)
		return
	}

	ch <- prometheus.MustNewConstMetric(up, prometheus.GaugeValue, 1)
	level.Info(logger).Log("msg", "collect succeeded", "host", c.hostURL, "coverted_metrics", converted, "failed_metrics", failed)
}

func probeFailure(start time.Time, msg string, err error, url string) {
	level.Warn(logger).Log("msg", msg, "err", err, "URL", url)
	probeFailureCount.Inc()
	probeDurationCount.Add(time.Since(start).Seconds())
}

func probeHandler(w http.ResponseWriter, r *http.Request) {
	requestURL := r.URL
	start := time.Now()
	level.Info(logger).Log("msg", "probe started", "URL", requestURL.String())

	// If a timeout is configured via the Prometheus header, add it to the request.
	if v := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"); v != "" {
		var err error
		timeoutSeconds, err = strconv.ParseFloat(v, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse timeout from Prometheus header: %s", err), http.StatusInternalServerError)
			probeFailure(start, "can't parse value of header X-Prometheus-Scrape-Timeout-Seconds", err, requestURL.String())
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds*float64(time.Second)))
	defer cancel()

	r = r.WithContext(ctx)

	query := requestURL.Query()
	if len(query) > 1 {
		http.Error(w, "Request should contain only one parameter: 'target'. Encode the URL if needed.", http.StatusBadRequest)
		probeFailure(start, "more than one parameter found in the request URL", nil, requestURL.String())
		return
	}
	target := query.Get("target")
	if target == "" {
		http.Error(w, "Parameter 'target' is missing", http.StatusBadRequest)
		probeFailure(start, "parameter 'target' is missing", nil, requestURL.String())
		return
	}

	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		http.Error(w, "Failed to create a request", http.StatusInternalServerError)
		probeFailure(start, "failed to create a request", err, requestURL.String())
		return
	}
	req = req.WithContext(ctx)
	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to execute a request", http.StatusBadGateway)
		probeFailure(start, "failed to execute a request", err, requestURL.String())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Server returned wrong response code", http.StatusBadGateway)
		probeFailure(start, "HTTP response status code is not 200", fmt.Errorf("HTTP status code is: %v, expected '200 OK'", resp.StatusCode), requestURL.String())
		return
	}

	// Wrap response body into buffered scanner
	s := bufio.NewScanner(resp.Body)

	registry := prometheus.NewRegistry()
	c := CommonStatusExporter{
		hostURL:        target,
		metricsScanner: s,
		startTime:      start,
	}
	registry.MustRegister(c)
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	h.ServeHTTP(w, r)

	duration := time.Since(start).Seconds()
	probeSuccessCount.Inc()
	probeDurationCount.Add(duration)
	level.Info(logger).Log("msg", "probe succeeded", "URL", requestURL.String(), "duration", fmt.Sprintf("%.2f s.", duration))
}

func main() {
	// TODO: CI + deploy
	// TODO: Development env
	// TODO: Development manual
	// TODO: Documentation + demo setup
	http.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		probeHandler(w, r)
	})
	http.Handle("/metrics", promhttp.Handler())

	port := getEnv("CS_PORT", "9259")
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		level.Error(logger).Log("msg", "failed to start the server", "err", err)
		os.Exit(1)
	}
}
