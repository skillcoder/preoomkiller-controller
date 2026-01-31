# https://hub.docker.com/_/golang/tags?name=1.25.6-alpine3.23
FROM golang:1.25.6-alpine3.23 AS builder

WORKDIR /build
COPY go.mod ./
RUN go mod download

COPY ./ ./
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o ./memory-leaker .

FROM scratch

COPY --from=builder /build/memory-leaker /bin/memory-leaker

EXPOSE 8080

ENTRYPOINT ["/bin/memory-leaker"]
