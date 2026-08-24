package httpapi

import (
	"context"
	"net"
	"net/http"
	"time"
)

func NewServer(handler http.Handler) *http.Server {
	return &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second}
}

func Shutdown(server *http.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}

func Listen(addr string) (net.Listener, error) { return net.Listen("tcp", addr) }
