package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
)

type JSONStore struct {
	mu   sync.RWMutex
	path string
	data snapshot
}

func Open(path string) (*JSONStore, error) {
	s := &JSONStore{path: path, data: emptySnapshot()}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *JSONStore) load() error {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取快照失败：%w", err)
	}
	var data snapshot
	if err := json.Unmarshal(b, &data); err != nil {
		return fmt.Errorf("快照 JSON 损坏：%w", err)
	}
	if data.Version != SnapshotVersion {
		return fmt.Errorf("不支持的快照版本：%d", data.Version)
	}
	if data.Cases == nil || data.Operations == nil {
		return errors.New("快照缺少必要集合")
	}
	for id, c := range data.Cases {
		if c == nil || id == "" || c.ID != id || c.Revision < 1 {
			return fmt.Errorf("快照包含无效个案：%s", id)
		}
		if err := domain.ValidateAggregate(c); err != nil {
			return fmt.Errorf("个案 %s 恢复校验失败：%w", id, err)
		}
	}
	if err := validateUniqueTreeCodes(data.Cases); err != nil {
		return err
	}
	if err := validateSnapshotRelations(data); err != nil {
		return err
	}
	s.data = data
	return nil
}

func (s *JSONStore) Get(id string) (*domain.MigrationCase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.data.Cases[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return cloneCase(c)
}

func (s *JSONStore) List() ([]*domain.MigrationCase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.data.Cases))
	for id := range s.data.Cases {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]*domain.MigrationCase, 0, len(ids))
	for _, id := range ids {
		result = append(result, s.data.Cases[id])
	}
	return result, nil
}

func (s *JSONStore) FindByTreeCode(normalized, excludeCaseID string) (*domain.MigrationCase, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for id, c := range s.data.Cases {
		if id != excludeCaseID && domain.NormalizeTreeCode(c.Plan.TreeCode) == normalized {
			copyCase, err := cloneCase(c)
			return copyCase, true, err
		}
	}
	return nil, false, nil
}

func (s *JSONStore) LookupOperation(requestID string) (OperationResult, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result, ok := s.data.Operations[requestID]
	if ok {
		result.Response = append(json.RawMessage(nil), result.Response...)
	}
	return result, ok, nil
}

func (s *JSONStore) Create(c *domain.MigrationCase, requestID, action string, response json.RawMessage, now time.Time) error {
	return s.CreateWithAudit(c, requestID, action, response, now, AuditContext{Actor: "系统角色", Result: "创建迁移方案草稿"})
}

func (s *JSONStore) CreateWithAudit(c *domain.MigrationCase, requestID, action string, response json.RawMessage, now time.Time, audit AuditContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data.Cases[c.ID]; exists {
		return fmt.Errorf("个案 %s 已存在", c.ID)
	}
	if _, exists := s.data.Operations[requestID]; exists {
		return fmt.Errorf("request_id 已被使用")
	}
	if conflict := findTreeCodeConflict(s.data.Cases, c.Plan.TreeCode, c.ID); conflict != nil {
		return conflict
	}
	copyCase, err := cloneCase(c)
	if err != nil {
		return err
	}
	s.data.Cases[c.ID] = copyCase
	s.record(requestID, action, domain.Status(""), c, response, now, audit)
	if committed, err := s.persistLocked(); err != nil {
		if !committed {
			delete(s.data.Cases, c.ID)
			delete(s.data.Operations, requestID)
			s.data.Events = s.data.Events[:len(s.data.Events)-1]
		}
		return err
	}
	return nil
}

func (s *JSONStore) Save(c *domain.MigrationCase, expected int64, requestID, action string, from domain.Status, response json.RawMessage, now time.Time) error {
	return s.SaveWithAudit(c, expected, requestID, action, from, response, now, AuditContext{Actor: "系统角色", Result: action})
}

func (s *JSONStore) SaveWithAudit(c *domain.MigrationCase, expected int64, requestID, action string, from domain.Status, response json.RawMessage, now time.Time, audit AuditContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.data.Operations[requestID]; ok {
		if existing.Action == action {
			return errOperationExists
		}
		return fmt.Errorf("request_id 已用于其他操作")
	}
	current, ok := s.data.Cases[c.ID]
	if !ok {
		return domain.ErrNotFound
	}
	if current.Revision != expected {
		return &domain.RevisionConflict{Expected: expected, Current: current.Revision}
	}
	if conflict := findTreeCodeConflict(s.data.Cases, c.Plan.TreeCode, c.ID); conflict != nil {
		return conflict
	}
	copyCase, err := cloneCase(c)
	if err != nil {
		return err
	}
	previous := current
	s.data.Cases[c.ID] = copyCase
	s.record(requestID, action, from, c, response, now, audit)
	if committed, err := s.persistLocked(); err != nil {
		if !committed {
			s.data.Cases[c.ID] = previous
			delete(s.data.Operations, requestID)
			s.data.Events = s.data.Events[:len(s.data.Events)-1]
		}
		return err
	}
	return nil
}

func (s *JSONStore) Events(caseID string) ([]AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]AuditEvent, 0)
	for _, event := range s.data.Events {
		if caseID == "" || event.CaseID == caseID {
			result = append(result, event)
		}
	}
	return result, nil
}

func (s *JSONStore) record(requestID, action string, from domain.Status, c *domain.MigrationCase, response json.RawMessage, now time.Time, audit AuditContext) {
	s.data.Operations[requestID] = OperationResult{RequestID: requestID, Action: action, CaseID: c.ID, Revision: c.Revision, Response: append(json.RawMessage(nil), response...), CreatedAt: now}
	sequence := int64(len(s.data.Events) + 1)
	s.data.Events = append(s.data.Events, AuditEvent{ID: fmt.Sprintf("event-%06d", sequence), Sequence: sequence, CaseID: c.ID, Action: action, From: from, To: c.Status, Revision: c.Revision, RequestID: requestID, Actor: audit.Actor, Result: audit.Result, Timestamp: now})
}

func cloneCase(c *domain.MigrationCase) (*domain.MigrationCase, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var result domain.MigrationCase
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *JSONStore) Path() string { return filepath.Clean(s.path) }

func validateSnapshotRelations(data snapshot) error {
	lastRevision := make(map[string]int64)
	for index, event := range data.Events {
		c, ok := data.Cases[event.CaseID]
		if !ok {
			return fmt.Errorf("审计事件 %s 引用了不存在的个案", event.ID)
		}
		if event.Revision < lastRevision[event.CaseID] || event.Revision > c.Revision {
			return fmt.Errorf("审计事件 %s 的 revision 顺序无效", event.ID)
		}
		if event.Sequence != 0 && event.Sequence != int64(index+1) {
			return fmt.Errorf("审计事件 %s 的序号无效", event.ID)
		}
		if event.Actor == "" || event.Result == "" {
			return fmt.Errorf("审计事件 %s 缺少责任主体或业务结果", event.ID)
		}
		lastRevision[event.CaseID] = event.Revision
		operation, ok := data.Operations[event.RequestID]
		if !ok || operation.CaseID != event.CaseID || operation.Revision != event.Revision {
			return fmt.Errorf("审计事件 %s 缺少匹配的幂等结果", event.ID)
		}
	}
	for requestID, operation := range data.Operations {
		if requestID == "" || operation.RequestID != requestID {
			return fmt.Errorf("幂等结果键 %s 无效", requestID)
		}
		if _, ok := data.Cases[operation.CaseID]; !ok {
			return fmt.Errorf("幂等结果 %s 引用了不存在的个案", requestID)
		}
		var result domain.MigrationCase
		if err := json.Unmarshal(operation.Response, &result); err != nil || result.ID != operation.CaseID || result.Revision != operation.Revision {
			return fmt.Errorf("幂等结果 %s 的响应快照无效", requestID)
		}
	}
	return nil
}

func findTreeCodeConflict(cases map[string]*domain.MigrationCase, treeCode, excludeCaseID string) error {
	normalized := domain.NormalizeTreeCode(treeCode)
	for id, c := range cases {
		if id != excludeCaseID && domain.NormalizeTreeCode(c.Plan.TreeCode) == normalized {
			return &domain.TreeCodeConflict{CaseID: id, Status: c.Status}
		}
	}
	return nil
}

func validateUniqueTreeCodes(cases map[string]*domain.MigrationCase) error {
	seen := make(map[string]*domain.MigrationCase, len(cases))
	for _, c := range cases {
		key := domain.NormalizeTreeCode(c.Plan.TreeCode)
		if existing, ok := seen[key]; ok {
			return fmt.Errorf("快照中古树编号冲突：%s 与 %s", existing.ID, c.ID)
		}
		seen[key] = c
	}
	return nil
}
