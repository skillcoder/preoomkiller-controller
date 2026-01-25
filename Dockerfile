# https://hub.docker.com/_/golang/tags?name=1.25.6-alpine3.23
FROM golang:1.25.6-alpine3.23 as builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY ./cmd/ ./cmd/
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o ./preoomkiller-controller ./cmd/preoomkiller-controller

# Starting on Scratch
FROM scratch

# Moving needed binaries to
COPY --from=builder /build/preoomkiller-controller /bin/preoomkiller-controller

ENTRYPOINT ["/bin/preoomkiller-controller"]
