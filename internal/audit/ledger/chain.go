package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func ComputeDigest(event Event) (string, error) {
	material := eventHashMaterial{SchemaVersion: event.SchemaVersion, Sequence: event.Sequence, AggregateID: event.AggregateID, AggregateVersion: event.AggregateVersion, Type: event.Type, Actor: event.Actor, At: event.At, IdempotencyKey: event.IdempotencyKey, RequestDigest: event.RequestDigest, PreviousDigest: event.PreviousDigest, Projection: event.Projection, Response: event.Response}
	b, err := json.Marshal(material)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func Seal(event *Event) error {
	digest, err := ComputeDigest(*event)
	if err != nil {
		return err
	}
	event.Digest = digest
	return nil
}

func VerifyChain(events []Event) error {
	previous := ""
	for i, event := range events {
		if event.Sequence != int64(i+1) {
			return fmt.Errorf("审计序号不连续：位置 %d 的序号为 %d", i, event.Sequence)
		}
		if event.PreviousDigest != previous {
			return fmt.Errorf("审计事件 %d 的前序摘要不匹配", event.Sequence)
		}
		digest, err := ComputeDigest(event)
		if err != nil {
			return err
		}
		if digest != event.Digest {
			return fmt.Errorf("审计事件 %d 内容摘要不匹配", event.Sequence)
		}
		previous = event.Digest
	}
	return nil
}
