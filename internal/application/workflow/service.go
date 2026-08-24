package workflow

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"sensor-calibration-release/internal/policy/evaluation"
	"sensor-calibration-release/internal/storage/jsonstore"
)

type Service struct {
	store     *jsonstore.Store
	evaluator *evaluation.Evaluator
	now       func() time.Time
	newID     func(string) string
}

func New(store *jsonstore.Store) *Service {
	return &Service{store: store, evaluator: evaluation.New(), now: time.Now, newID: randomID}
}

func randomID(prefix string) string {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("生成随机标识失败: %v", err))
	}
	return prefix + "_" + hex.EncodeToString(b)
}

func stableBatchID(actor, key string) string {
	h := sha256.Sum256([]byte(actor + "\x00" + key))
	return "batch_" + hex.EncodeToString(h[:10])
}

func validateMeta(meta CommandMeta) error {
	if meta.Actor == "" {
		return fmt.Errorf("actor 不能为空")
	}
	if meta.IdempotencyKey == "" {
		return fmt.Errorf("idempotencyKey 不能为空")
	}
	if meta.ExpectedVersion < 0 {
		return fmt.Errorf("expectedVersion 不能为负数")
	}
	return nil
}

func requestFingerprint(value any) (string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(b)
	return hex.EncodeToString(digest[:]), nil
}

func decodeMutation(result jsonstore.CommitResult) (MutationResponse, error) {
	var response MutationResponse
	if err := json.Unmarshal(result.Response, &response); err != nil {
		return response, err
	}
	response.Replayed = result.Replayed
	return response, nil
}

func (s *Service) Store() *jsonstore.Store { return s.store }
