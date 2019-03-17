FROM golang:1.12.1 as base

WORKDIR /go/src/testserver
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /go/bin/testserver .

FROM scratch

COPY --from=base /go/bin/testserver /testserver
COPY valid_metrics.txt /valid_metrics.txt

ENTRYPOINT ["/testserver"]
