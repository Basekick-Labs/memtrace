FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o memtrace ./cmd/memtrace/

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /build/memtrace .
COPY --from=builder /build/memtrace.toml .

RUN mkdir -p /app/data

EXPOSE 9100

ENTRYPOINT ["./memtrace"]
