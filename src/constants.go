package logsQueryExec

import "errors"

const (
	ResponseStatusSuccess = "success"
	ResponseStatusRunning = "running"
	ResponseStatusFailed  = "failed"
)

var (
	ErrorEnableRetry = errors.New("enable retry")
)
