package ledger

import (
	"encoding/json"
	"testing"
	"time"
)

func TestVerifyChainDetectsTampering(t *testing.T) {
	first := Event{SchemaVersion: 1, Sequence: 1, AggregateID: "batch-1", AggregateVersion: 1, Type: "batch.created", Actor: "tech", At: time.Unix(1, 0).UTC(), Projection: json.RawMessage(`{"schemaVersion":1}`)}
	if err := Seal(&first); err != nil {
		t.Fatal(err)
	}
	second := Event{SchemaVersion: 1, Sequence: 2, AggregateID: "batch-1", AggregateVersion: 2, Type: "sensor.registered", Actor: "tech", At: time.Unix(2, 0).UTC(), PreviousDigest: first.Digest, Projection: json.RawMessage(`{"schemaVersion":1}`)}
	if err := Seal(&second); err != nil {
		t.Fatal(err)
	}
	if err := VerifyChain([]Event{first, second}); err != nil {
		t.Fatalf("有效链被拒绝: %v", err)
	}
	second.Actor = "attacker"
	if err := VerifyChain([]Event{first, second}); err == nil {
		t.Fatal("篡改操作者后应校验失败")
	}
}

func TestVerifyChainDetectsTruncationSequence(t *testing.T) {
	event := Event{SchemaVersion: 1, Sequence: 2, AggregateID: "batch-1", AggregateVersion: 1, Type: "batch.created", Actor: "tech", At: time.Unix(1, 0).UTC(), Projection: json.RawMessage(`{}`)}
	if err := Seal(&event); err != nil {
		t.Fatal(err)
	}
	if err := VerifyChain([]Event{event}); err == nil {
		t.Fatal("从序号 2 开始的账本应被拒绝")
	}
}
