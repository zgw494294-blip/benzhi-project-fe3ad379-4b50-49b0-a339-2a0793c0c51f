package eventledgeralias_test

import (
	"testing"

	"sensor-calibration-release/internal/application/workflow"
	"sensor-calibration-release/internal/storage/jsonstore"
)

func TestEventQueryDoesNotExposeLedgerSlice(t *testing.T) {
	store, err := jsonstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(store)
	created, err := service.CreateBatch(workflow.CreateBatchCommand{
		CommandMeta: workflow.CommandMeta{Actor: "tech-a", ExpectedVersion: 0, IdempotencyKey: "create-1"},
		StationCode: "ST-ALIAS",
		Title:       "事件所有权测试",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.BatchID == "" {
		t.Fatal("建批未返回批次标识")
	}

	events := store.Events("")
	if len(events) != 1 {
		t.Fatalf("期望查询到一个事件，实际为 %d", len(events))
	}
	// 查询结果是调用方的快照，修改其元数据不应改写 Store 内部账本。
	events[0].Sequence = 99
	if len(events[0].Projection) == 0 {
		t.Fatal("事件缺少投影载荷")
	}
	events[0].Projection[0] = 'X'
	if _, err := store.Verify(); err != nil {
		t.Fatalf("修改查询结果不应污染账本: %v", err)
	}
}
