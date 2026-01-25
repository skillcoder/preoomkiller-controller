package httpserver

import "time"

const (
	defaultPort = "8080"

	readTimeout       = 3 * time.Second
	readHeaderTimeout = 3 * time.Second
	writeTimeout      = 5 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 1 << 12 // 4kb
)
