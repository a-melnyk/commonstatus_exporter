package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	up = prometheus.NewDesc(
		"consul_up",
		"Was talking to Consul successful.",
		nil, nil,
	)
)

type CommonStatusExporter struct {
	hostURL string
}

// Implements prometheus.Collector.
func (c CommonStatusExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
}

func (c CommonStatusExporter) Collect(ch chan<- prometheus.Metric) {
	floatHost, _ := strconv.ParseFloat(c.hostURL, 64)
	ch <- prometheus.MustNewConstMetric(up, prometheus.GaugeValue, floatHost)

	// scrape metrics from the hostURL and convert them using the convert package
}

func probeHandler(w http.ResponseWriter, r *http.Request) {
	timeoutSeconds := 10.0
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds*float64(time.Second)))
	defer cancel()
	r = r.WithContext(ctx)

	probeSuccessGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_success",
		Help: "Displays whether or not the probe was a success",
	})
	probeDurationGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_duration_seconds",
		Help: "Returns how long the probe took to complete in seconds",
	})

	params := r.URL.Query()
	target := params.Get("target")
	if target == "" {
		http.Error(w, "Target parameter is missing", http.StatusBadRequest)
		return
	}

	// sl := newScrapeLogger(logger, moduleName, target)
	// level.Info.Log("msg", "Beginning probe", "probe", module.Prober, "timeout_seconds", timeoutSeconds)

	// start := time.Now()
	registry := prometheus.NewRegistry()
	registry.MustRegister(probeSuccessGauge)
	registry.MustRegister(probeDurationGauge)
	c := CommonStatusExporter{
		hostURL: target,
	}
	registry.MustRegister(c)

	// duration := time.Since(start).Seconds()
	// probeDurationGauge.Set(duration)
	// if success {
	// 	probeSuccessGauge.Set(1)
	// 	level.Info(sl).Log("msg", "Probe succeeded", "duration_seconds", duration)
	// } else {
	// 	level.Error(sl).Log("msg", "Probe failed", "duration_seconds", duration)
	// }
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}

func main() {
	http.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		probeHandler(w, r)
	})
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":8080", nil))
}
