package application

import (
	"fmt"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/store"
)

func (s *Service) CreateCase(cmd CreateCaseCommand) (*domain.MigrationCase, error) {
	const action = "create_case"
	digest, err := digestPayload(cmd.Plan)
	if err != nil {
		return nil, err
	}
	if replay, ok, err := s.replay(cmd.RequestID, action, digest); ok || err != nil {
		return replay, err
	}
	now := s.now().UTC()
	c, err := domain.NewCase(s.newID("case"), cmd.Plan, now)
	if err != nil {
		return nil, err
	}
	if err := s.ensureTreeCodeAvailable(c.Plan.TreeCode, ""); err != nil {
		return nil, err
	}
	response, err := marshalCase(c)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateWithAudit(c, cmd.RequestID, action, digest, response, now, store.AuditContext{Actor: "系统角色：方案编制", Result: "创建古树迁移方案草稿"}); err != nil {
		if replay, ok, replayErr := s.replay(cmd.RequestID, action, digest); ok {
			return replay, nil
		} else if replayErr != nil {
			return nil, replayErr
		}
		return nil, err
	}
	return c, nil
}

func (s *Service) UpdateDraft(id string, cmd UpdateDraftCommand) (*domain.MigrationCase, error) {
	const action = "update_draft"
	digest, err := digestPayload(cmd.Plan)
	if err != nil {
		return nil, err
	}
	c, replayed, err := s.loadForMutation(id, cmd.CommandMeta, action, digest)
	if err != nil || replayed {
		return c, err
	}
	if c.Status != domain.StatusDraft {
		return nil, fmt.Errorf("%w：仅 draft 可编辑", domain.ErrInvalidTransition)
	}
	if err := domain.ValidateDraft(cmd.Plan); err != nil {
		return nil, err
	}
	if err := s.ensureTreeCodeAvailable(cmd.Plan.TreeCode, c.ID); err != nil {
		return nil, err
	}
	expected, from, now := c.Revision, c.Status, s.now().UTC()
	c.Plan = cmd.Plan
	c.Revision++
	c.UpdatedAt = now
	c.Timeline = append(c.Timeline, domain.TimelineEntry{Status: c.Status, Revision: c.Revision, Action: "更新草稿资料", OccurredAt: now})
	response, err := marshalCase(c)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveWithAudit(c, expected, cmd.RequestID, action, digest, from, response, now, store.AuditContext{Actor: "系统角色：方案编制", Result: "更新草稿资料并保留古树编号展示值"}); err != nil {
		return s.replayAfterConflict(cmd.RequestID, action, digest, err)
	}
	return c, nil
}

func (s *Service) replayAfterConflict(requestID, action, digest string, saveErr error) (*domain.MigrationCase, error) {
	if replay, ok, err := s.replay(requestID, action, digest); ok {
		return replay, nil
	} else if err != nil {
		return nil, err
	}
	return nil, saveErr
}
