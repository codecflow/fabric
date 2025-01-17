# Build stage
FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Debugging stage
# Commenting out FROM scratch for debugging purposes
# FROM scratch

# Use a base image that supports debugging
FROM alpine

WORKDIR /app

COPY --from=builder /app/main .

EXPOSE 9000

# Keep the container running for debugging
CMD ["sh", "-c", "./main & tail -f /dev/null"]