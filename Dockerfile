# https://hub.docker.com/_/golang/tags?name=1.25.6-alpine3.23
FROM golang:1.25.6-alpine3.23 AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY ./cmd/ ./cmd/
COPY ./internal/ ./internal/
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o ./preoomkiller-controller ./cmd/preoomkiller-controller


# Stage with tzdata for zoneinfo (version explicit for reproducible builds)
FROM alpine:3.23.3 AS tzdata
# https://pkgs.alpinelinux.org/packages?name=tzdata&branch=v3.23&repo=&arch=x86_64
RUN apk --no-cache add \
    tzdata=2025c-r0


# Final image: scratch + binary + zoneinfo
FROM scratch
ENV ZONEINFO=/usr/share/zoneinfo
COPY --from=tzdata /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /build/preoomkiller-controller /bin/preoomkiller-controller
ENTRYPOINT ["/bin/preoomkiller-controller"]
