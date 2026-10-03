FROM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# No -s or -w: the OpenTelemetry Go eBPF agent needs the symbol table to
# find the functions it hooks, and DWARF to find struct field offsets for
# Go versions it has no cached offsets for (Go 1.27 in agent v0.24.0).
RUN CGO_ENABLED=0 GOOS=linux go build -o book-review-publisher ./cmd/server

FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

RUN adduser -D -g "" appuser

COPY --from=builder /app/book-review-publisher /usr/local/bin/book-review-publisher

USER appuser

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/book-review-publisher"]
