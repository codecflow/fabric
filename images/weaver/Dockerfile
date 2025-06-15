FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o weaver ./cmd/weaver

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/weaver .
COPY --from=builder /app/config.yaml .

EXPOSE 8080

CMD ["./weaver"]
