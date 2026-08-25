package httpui

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/application"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
)

func (s *Server) Workbench(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	b, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "工作台资源不可用", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

func (s *Server) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
func (s *Server) GetRules(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.service.RuleCatalog())
}

func (s *Server) ListCases(w http.ResponseWriter, r *http.Request) {
	query, err := parseListQuery(r)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := s.service.QueryCases(query)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func parseListQuery(r *http.Request) (application.ListCasesQuery, error) {
	values := r.URL.Query()
	query := application.ListCasesQuery{TreeCode: values.Get("tree_code"), Species: values.Get("species"), Location: values.Get("location"), Sort: values.Get("sort"), Page: 1, PageSize: 20}
	for _, value := range values["status"] {
		for _, status := range strings.Split(value, ",") {
			if status = strings.TrimSpace(status); status != "" {
				query.Statuses = append(query.Statuses, domain.Status(status))
			}
		}
	}
	var err error
	if value := values.Get("page"); value != "" {
		query.Page, err = strconv.Atoi(value)
		if err != nil {
			return query, queryError("page", "须为整数")
		}
	}
	if value := values.Get("page_size"); value != "" {
		query.PageSize, err = strconv.Atoi(value)
		if err != nil {
			return query, queryError("page_size", "须为整数")
		}
	}
	return query, nil
}

func queryError(field, message string) error {
	errs := domain.NewValidationErrors()
	errs.Add(field, message)
	return errs
}

func (s *Server) GetCase(w http.ResponseWriter, r *http.Request) {
	id, err := cleanID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	detail, err := s.service.GetCase(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) CreateCase(w http.ResponseWriter, r *http.Request) {
	var cmd application.CreateCaseCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	c, err := s.service.CreateCase(cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	if r.Context().Err() != nil {
		writeError(w, &requestError{http.StatusRequestTimeout, "request_canceled", "请求已取消"})
		return
	}
	w.Header().Set("Location", "/api/cases/"+c.ID)
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) UpdateDraft(w http.ResponseWriter, r *http.Request) {
	id, err := cleanID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var cmd application.UpdateDraftCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	c, err := s.service.UpdateDraft(id, cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) ValidateCase(w http.ResponseWriter, r *http.Request) {
	id, err := cleanID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var cmd application.ValidateCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	c, err := s.service.ValidateCase(id, cmd)
	if err != nil && errors.Is(err, domain.ErrBlockingFindings) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"case": c, "error": errorBody{Code: "blocking_findings", Message: err.Error()}})
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) StartReview(w http.ResponseWriter, r *http.Request) {
	id, cmd, ok := decodeMutation[application.StartReviewCommand](w, r)
	if !ok {
		return
	}
	c, err := s.service.StartReview(id, cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) SubmitOpinion(w http.ResponseWriter, r *http.Request) {
	id, cmd, ok := decodeMutation[application.SubmitOpinionCommand](w, r)
	if !ok {
		return
	}
	c, err := s.service.SubmitOpinion(id, cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) SubmitRevision(w http.ResponseWriter, r *http.Request) {
	id, cmd, ok := decodeMutation[application.SubmitRevisionCommand](w, r)
	if !ok {
		return
	}
	c, err := s.service.SubmitRevision(id, cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) ApproveCase(w http.ResponseWriter, r *http.Request) {
	id, cmd, ok := decodeMutation[application.ApproveCommand](w, r)
	if !ok {
		return
	}
	c, err := s.service.Approve(id, cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func decodeMutation[T any](w http.ResponseWriter, r *http.Request) (string, T, bool) {
	var cmd T
	id, err := cleanID(r)
	if err != nil {
		writeError(w, err)
		return "", cmd, false
	}
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return "", cmd, false
	}
	return id, cmd, true
}
