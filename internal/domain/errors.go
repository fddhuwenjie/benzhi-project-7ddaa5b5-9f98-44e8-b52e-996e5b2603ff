package domain

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound            = errors.New("个案不存在")
	ErrAlreadyApproved     = errors.New("已批准个案不可修改")
	ErrInvalidTransition   = errors.New("状态流转不合法")
	ErrIncompleteReviews   = errors.New("两个专业审查席位尚未全部完成")
	ErrUnresolvedIssues    = errors.New("仍有未解决的审查问题")
	ErrBlockingFindings    = errors.New("仍有阻断性技术规则结果")
	ErrDuplicateDiscipline = errors.New("该专业席位已提交意见")
)

type ValidationErrors struct {
	Fields map[string]string `json:"fields"`
}

func (e *ValidationErrors) Error() string { return "字段校验失败" }

func NewValidationErrors() *ValidationErrors {
	return &ValidationErrors{Fields: make(map[string]string)}
}

func (e *ValidationErrors) Add(field, message string) { e.Fields[field] = message }
func (e *ValidationErrors) Empty() bool               { return len(e.Fields) == 0 }

type RevisionConflict struct {
	Expected int64 `json:"expected"`
	Current  int64 `json:"current"`
}

type TreeCodeConflict struct {
	CaseID string `json:"case_id"`
	Status Status `json:"status"`
}

type IdempotencyConflict struct {
	RequestID string `json:"request_id"`
	Action    string `json:"action"`
}

func (e *TreeCodeConflict) Error() string {
	return fmt.Sprintf("古树编号已被个案 %s 使用（状态：%s）", e.CaseID, e.Status)
}

func (e *RevisionConflict) Error() string {
	return fmt.Sprintf("revision 冲突：期望 %d，当前 %d", e.Expected, e.Current)
}

func (e *IdempotencyConflict) Error() string {
	return fmt.Sprintf("request_id %s 已用于操作 %s 但请求载荷不同", e.RequestID, e.Action)
}
