FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.work go.work.sum  ./
#todo: can this be removed?
COPY weaver/ weaver/
COPY shuttle/ shuttle/

RUN go mod download
RUN go mod verify

COPY config/dev.shuttle.yaml config.yaml

RUN CGO_ENABLED=0 GOOS=linux go build -o shuttle -v shuttle/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/shuttle .
COPY --from=builder /app/config.yaml .

EXPOSE 8080

CMD ["./shuttle", "--config", "config.yaml"]