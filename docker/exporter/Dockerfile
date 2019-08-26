FROM golang:1.12 as builder
ADD . /app
WORKDIR /app
# COPY go.mod go.sum ./
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cs-exporter .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/cs-exporter .
EXPOSE 9259
CMD ["./cs-exporter"]
