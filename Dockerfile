# Build context is the plugins/ directory so we can access the local SDK.
FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY plugin-sdk-go /plugin-sdk-go
COPY loki-plugin/go.mod loki-plugin/go.sum ./
RUN go mod download

COPY loki-plugin/ .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /plugin ./cmd/plugin

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
COPY --from=builder /plugin /plugin

EXPOSE 50051
ENTRYPOINT ["/plugin"]
