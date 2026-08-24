package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"sensor-calibration-release/internal/application/workflow"
	"sensor-calibration-release/internal/storage/jsonstore"
	"sensor-calibration-release/internal/transport/httpapi"
)

func main() {
	addr := flag.String("addr", defaultAddr(), "监听地址")
	dataDir := flag.String("data", "", "持久化目录，默认 ./data")
	selfcheck := flag.Bool("selfcheck", false, "运行真实网络自检后退出")
	timeout := flag.Duration("selfcheck-timeout", 15*time.Second, "自检超时")
	flag.Parse()
	if err := run(*addr, *dataDir, *selfcheck, *timeout); err != nil {
		log.Fatal(err)
	}
}

func defaultAddr() string {
	if port := os.Getenv("PORT"); port != "" {
		return "127.0.0.1:" + port
	}
	return "127.0.0.1:19081"
}

func run(addr, dataDir string, selfcheck bool, timeout time.Duration) error {
	if addr == "" {
		return fmt.Errorf("监听地址不能为空")
	}
	cleanup := func() {}
	if dataDir == "" {
		if selfcheck {
			tmp, err := os.MkdirTemp("", "sensor-calibration-selfcheck-")
			if err != nil {
				return err
			}
			dataDir, cleanup = tmp, func() { _ = os.RemoveAll(tmp) }
		} else {
			dataDir = filepath.Join(".", "data")
		}
	}
	defer cleanup()
	store, err := jsonstore.Open(dataDir)
	if err != nil {
		return err
	}
	service := workflow.New(store)
	server := httpapi.NewServer(httpapi.New(service).Handler())
	listener, err := httpapi.Listen(addr)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", addr, err)
	}
	if selfcheck {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := httpapi.RunSelfcheck(ctx, listener, server); err != nil {
			return fmt.Errorf("selfcheck 失败: %w", err)
		}
		log.Printf("selfcheck 通过，监听地址 %s", listener.Addr())
		return nil
	}
	log.Printf("校准放行服务监听 %s，数据目录 %s", listener.Addr(), dataDir)
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case <-signals:
		return httpapi.Shutdown(server)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
