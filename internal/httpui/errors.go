package httpui

import (
	"errors"
	"net/http"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
)

type requestError struct {
	status        int
	code, message string
}

func (e *requestError) Error() string { return e.message }

func writeError(w http.ResponseWriter, err error) {
	body := errorBody{Code: "internal_error", Message: "服务处理失败"}
	status := http.StatusInternalServerError
	var reqErr *requestError
	var validation *domain.ValidationErrors
	var conflict *domain.RevisionConflict
	var treeConflict *domain.TreeCodeConflict
	var idempotency *domain.IdempotencyConflict
	switch {
	case errors.As(err, &reqErr):
		status, body.Code, body.Message = reqErr.status, reqErr.code, reqErr.message
	case errors.As(err, &validation):
		status, body.Code, body.Message, body.Fields = http.StatusUnprocessableEntity, "validation_failed", validation.Error(), validation.Fields
	case errors.As(err, &conflict):
		status, body.Code, body.Message, body.Current = http.StatusConflict, "revision_conflict", conflict.Error(), conflict.Current
	case errors.As(err, &treeConflict):
		status, body.Code, body.Message = http.StatusConflict, "tree_code_conflict", treeConflict.Error()
		body.Fields = map[string]string{"tree_code": treeConflict.Error()}
		body.ConflictCaseID, body.ConflictStatus = treeConflict.CaseID, treeConflict.Status
	case errors.As(err, &idempotency):
		status, body.Code, body.Message = http.StatusConflict, "idempotency_conflict", idempotency.Error()
		body.Fields = map[string]string{"request_id": idempotency.Error()}
	case errors.Is(err, domain.ErrNotFound):
		status, body.Code, body.Message = http.StatusNotFound, "not_found", err.Error()
	case errors.Is(err, domain.ErrBlockingFindings):
		status, body.Code, body.Message = http.StatusUnprocessableEntity, "blocking_findings", err.Error()
	case errors.Is(err, domain.ErrUnresolvedIssues):
		status, body.Code, body.Message = http.StatusUnprocessableEntity, "unresolved_issues", err.Error()
	case errors.Is(err, domain.ErrInvalidTransition), errors.Is(err, domain.ErrAlreadyApproved), errors.Is(err, domain.ErrDuplicateDiscipline):
		status, body.Code, body.Message = http.StatusConflict, "business_conflict", err.Error()
	}
	writeJSON(w, status, errorEnvelope{Error: body})
}
