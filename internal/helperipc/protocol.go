package helperipc

import (
	"encoding/json"

	"winrouter/internal/core"
)

const (
	MethodValidate           = "validate"
	MethodApply              = "apply"
	MethodStop               = "stop"
	MethodStatus             = "status"
	MethodHeartbeat          = "heartbeat"
	MethodShutdown           = "shutdown"
	MethodFaultTerminateCore = "fault-terminate-core"
	MaxMessageSize           = 4 << 20
)

type Request struct {
	ID        string          `json:"id"`
	Method    string          `json:"method"`
	AuthToken string          `json:"auth_token"`
	Params    json.RawMessage `json:"params,omitempty"`
}

type FaultService interface {
	FaultTerminateCore() error
}
type Response struct {
	ID     string    `json:"id"`
	OK     bool      `json:"ok"`
	Error  *RPCError `json:"error,omitempty"`
	Result any       `json:"result,omitempty"`
}
type RPCError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type ConfigParams struct {
	Config json.RawMessage `json:"config"`
}
type Heartbeat struct {
	UnixMillis int64 `json:"unix_millis"`
}

type Service interface {
	Validate([]byte) error
	Apply([]byte) error
	Stop() error
	Status() core.Status
}
