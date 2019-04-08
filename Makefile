.PHONY: run
run:
	docker-compose up --build

build:
	docker build -t cs_exporter -f docker/exporter/Dockerfile .

test:
	go test -timeout 30s -v
