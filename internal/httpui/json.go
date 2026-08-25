package httpui

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
)

const maxRequestBytes = 1 << 20

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code           string            `json:"code"`
	Message        string            `json:"message"`
	Fields         map[string]string `json:"fields,omitempty"`
	Current        int64             `json:"current_revision,omitempty"`
	ConflictCaseID string            `json:"conflict_case_id,omitempty"`
	ConflictStatus domain.Status     `json:"conflict_status,omitempty"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	contentType := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		return &requestError{http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type 必须为 application/json"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return &requestError{http.StatusRequestEntityTooLarge, "request_too_large", "请求正文不能超过 1 MiB"}
		}
		return &requestError{http.StatusBadRequest, "invalid_json", "JSON 请求无效：" + err.Error()}
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return &requestError{http.StatusBadRequest, "invalid_json", "请求正文只能包含一个 JSON 对象"}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func cleanID(r *http.Request) (string, error) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" || strings.ContainsAny(id, "/\\") || len(id) > 128 {
		return "", &requestError{http.StatusBadRequest, "invalid_id", "个案标识无效"}
	}
	return id, nil
}
