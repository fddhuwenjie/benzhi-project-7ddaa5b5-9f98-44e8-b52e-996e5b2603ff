package application

import (
	"encoding/json"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/store"
)

type Repository interface {
	Get(id string) (*domain.MigrationCase, error)
	List() ([]*domain.MigrationCase, error)
	FindByTreeCode(normalized, excludeCaseID string) (*domain.MigrationCase, bool, error)
	LookupOperation(requestID string) (store.OperationResult, bool, error)
	CreateWithAudit(c *domain.MigrationCase, requestID, action, requestDigest string, response json.RawMessage, now time.Time, audit store.AuditContext) error
	SaveWithAudit(c *domain.MigrationCase, expected int64, requestID, action, requestDigest string, from domain.Status, response json.RawMessage, now time.Time, audit store.AuditContext) error
	Events(caseID string) ([]store.AuditEvent, error)
}

type PolicyEvaluator interface {
	Evaluate(caseID string, revision int64, plan domain.Plan) []domain.PolicyFinding
	EvaluateAffected(caseID string, revision int64, plan domain.Plan, changes []domain.FieldChange, previous []domain.PolicyFinding) []domain.PolicyFinding
	EvaluateRevision(caseID string, revision int64, plan domain.Plan, changes []domain.FieldChange, previous []domain.PolicyFinding) domain.RevisionEvaluation
}

type IDGenerator func(prefix string) string
