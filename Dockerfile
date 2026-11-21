FROM golang:1.27-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o book-review-publisher ./cmd/server

FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf

RUN adduser -D -g "" appuser

COPY --from=builder /app/book-review-publisher /usr/local/bin/book-review-publisher

USER appuser

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/book-review-publisher"]
