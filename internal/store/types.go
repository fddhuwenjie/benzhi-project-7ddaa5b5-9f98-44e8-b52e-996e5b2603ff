package store

import (
	"encoding/json"
	"errors"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
)

const SnapshotVersion = 1

var errOperationExists = errors.New("request_id 操作已经完成")

type OperationResult struct {
	RequestID     string          `json:"request_id"`
	Action        string          `json:"action"`
	CaseID        string          `json:"case_id"`
	Revision      int64           `json:"revision"`
	RequestDigest string          `json:"request_digest"`
	Response      json.RawMessage `json:"response"`
	CreatedAt     time.Time       `json:"created_at"`
}

type AuditEvent struct {
	ID        string        `json:"id"`
	Sequence  int64         `json:"sequence"`
	CaseID    string        `json:"case_id"`
	Action    string        `json:"action"`
	From      domain.Status `json:"from_status,omitempty"`
	To        domain.Status `json:"to_status"`
	Revision  int64         `json:"revision"`
	RequestID string        `json:"request_id"`
	Actor     string        `json:"actor"`
	Result    string        `json:"result"`
	Timestamp time.Time     `json:"timestamp"`
}

type AuditContext struct {
	Actor  string
	Result string
}

type snapshot struct {
	Version    int                              `json:"version"`
	Cases      map[string]*domain.MigrationCase `json:"cases"`
	Operations map[string]OperationResult       `json:"operations"`
	Events     []AuditEvent                     `json:"events"`
}

func emptySnapshot() snapshot {
	return snapshot{Version: SnapshotVersion, Cases: make(map[string]*domain.MigrationCase), Operations: make(map[string]OperationResult), Events: make([]AuditEvent, 0)}
}
