FROM golang:1.12.1

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY ./ ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /go/bin/cs-exporter .

ENTRYPOINT ["/go/bin/cs-exporter"]
