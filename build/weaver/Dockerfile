FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.work go.work.sum  ./
COPY weaver/ weaver/
#todo: can this be removed?
COPY shuttle/ shuttle/

RUN go mod download
RUN go mod verify

COPY config/dev.weaver.yaml config.yaml

RUN CGO_ENABLED=0 GOOS=linux go build -o weaver -v weaver/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/weaver .
COPY --from=builder /app/config.yaml .

EXPOSE 8080

CMD ["./weaver", "--config", "config.yaml"]