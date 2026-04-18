FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /xq-monitor ./cmd/xq-monitor

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /xq-monitor .

VOLUME ["/app/snapshots"]
ENTRYPOINT ["./xq-monitor"]
