package main

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var up = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "application_host_up",
	Help: "Was talking to the application successful.",
})

func init() {
	prometheus.MustRegister(up)
}

type Metric struct {
	m        prometheus.Metric
	lastSeen time.Time
}

type Exporter struct {
	host string
	// core prometheus handler
	coreHandler http.Handler
	// list of metrics from previous run
	metrics map[string]Metric
}

func (e *Exporter) prepareMetrics() error {
	// make request to backend. TODO - specify timeouts + retries if needed
	resp, err := http.Get(e.host)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// set gauge vector with error codes or return some error
	}

	// Wrap response body into buffered scanner. Set split function to scanLines
	s := bufio.NewScanner(resp.Body)
	s.Split(bufio.ScanLines)

	timestamp := time.Now()
	_ = timestamp

	// iterate over lines
	for s.Scan() {
		log.Print(s.Text())
		// convert metrics
		// check if exist in metrics
		//     - if yes - set it + lastSeen = timestamp
		//     - if no - register and set + add to metrics
	}
	// check if errors occured during reading - e.g dropped connection or etc.
	if err := s.Err(); err != nil {

	}

	// if for metrics that are in metrics but not right now - unregister and remove from map
	return nil
}

func (e *Exporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := e.prepareMetrics()
	if err != nil {
		log.Print(err)
		e.coreHandler.ServeHTTP(w, r) // serve anyway
		return
	}

	up.Set(1) // mark exporter as working
	e.coreHandler.ServeHTTP(w, r)
}

func main() {
	e := &Exporter{
		host:        os.Getenv("CS_SERVER"),
		coreHandler: promhttp.Handler(),
		metrics:     make(map[string]Metric),
	}

	http.Handle("/metrics", e)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
