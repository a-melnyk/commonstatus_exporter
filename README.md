CI build status: [![CircleCI](https://circleci.com/gh/gips0n/commonstatus_exporter.svg?style=svg)](https://circleci.com/gh/gips0n/commonstatus_exporter)

# commonstatus_exporter
Prometheus CommonStatus blackbox exporter

## Usage

Docker image with the exporter is automatically built from master branch by CI and available at [https://quay.io/repository/gips0n/cs_exporter](https://quay.io/repository/gips0n/cs_exporter)

To pull the image run:

```bash
docker pull quay.io/gips0n/cs_exporter
```

## Configuration

To configure exporter use environment variables:

* CS_LOG_LEVEL - set log level, supported values: DEBUG, INFO, WARN, ERROR; default: INFO
* CS_CONNECTION_TIMEOUT - set connection timeout; default: 8 seconds
* CS_PORT - set port on which the exporter will run; default: 9259

### Connection timeout

Connection timeout occurs when the exporter sends requests to backends (from which it scapes metrics) and the backend takes too long to respond to a request. The value is controlled by:

1. "X-Prometheus-Scrape-Timeout-Seconds" header in probe-request - per probe, has the highest priority, configured on prometheus server side
2. CS_CONNECTION_TIMEOUT environment variable - default for all probes
3. Default value of CS_CONNECTION_TIMEOUT - default for all probes

## Development

Use `make` or `make run` or `docker-compose up --build` to run local development environment which consists of local [prometheus](http://localhost:9090) server, [commonstatus_exporter](http://localhost:9259/metrics) and [testservice](http://localhost:8081/) which is hosting [sample data](./docker/testservice/valid_metrics.txt).

On [the prometheus target page](http://localhost:9090/targets) you should see 3 targets: prometheus, exporter, testservice and information about the last scrape and scrape errors. On [the graph page](http://localhost:9090/graph) you can execute queries to retrieve metrics.
