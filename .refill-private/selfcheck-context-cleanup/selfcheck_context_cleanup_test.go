package selfcheckcontextcleanup

import (
	"context"
	"net"
	"testing"
	"time"

	"sensor-calibration-release/internal/application/workflow"
	"sensor-calibration-release/internal/storage/jsonstore"
	"sensor-calibration-release/internal/transport/httpapi"
)

func TestCanceledSelfcheckClosesListener(t *testing.T) {
	store, err := jsonstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	listener, err := httpapi.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := httpapi.NewServer(httpapi.New(workflow.New(store)).Handler())
	t.Cleanup(func() { _ = server.Close() })
	address := listener.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := httpapi.RunSelfcheck(ctx, listener, server); err == nil {
		t.Fatal("已取消的 selfcheck 应返回错误")
	}

	conn, dialErr := net.DialTimeout("tcp", address, 500*time.Millisecond)
	if conn != nil {
		_ = conn.Close()
	}
	if dialErr == nil {
		t.Fatal("TestCanceledSelfcheckClosesListener: context 取消返回后监听端口仍可连接")
	}
}
