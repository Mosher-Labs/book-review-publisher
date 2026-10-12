FROM golang:1.27-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o book-review-publisher ./cmd/server

FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

RUN adduser -D -g "" appuser

COPY --from=builder /app/book-review-publisher /usr/local/bin/book-review-publisher

USER appuser

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/book-review-publisher"]
