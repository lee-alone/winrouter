package helperipc

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Server struct {
	tokenHash    [sha256.Size]byte
	service      Service
	wait         sync.WaitGroup
	lastActivity atomic.Int64
	shutdown     chan struct{}
	shutdownOnce sync.Once
}

func NewServer(authToken string, service Service) (*Server, error) {
	if len(authToken) < 32 {
		return nil, errors.New("helper authentication token must contain at least 32 characters")
	}
	if service == nil {
		return nil, errors.New("helper service is required")
	}
	server := &Server{tokenHash: sha256.Sum256([]byte(authToken)), service: service, shutdown: make(chan struct{})}
	server.lastActivity.Store(time.Now().UnixMilli())
	return server, nil
}

func (s *Server) LastActivity() time.Time            { return time.UnixMilli(s.lastActivity.Load()) }
func (s *Server) ShutdownRequested() <-chan struct{} { return s.shutdown }

func (s *Server) Serve(listener net.Listener) error {
	defer s.wait.Wait()
	for {
		connection, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		s.wait.Add(1)
		go func() { defer s.wait.Done(); defer connection.Close(); s.handle(connection) }()
	}
}

func (s *Server) handle(connection net.Conn) {
	_ = connection.SetDeadline(time.Now().Add(30 * time.Second))
	data, err := readFrame(connection)
	if err != nil {
		_ = writeFrame(connection, Response{OK: false, Error: &RPCError{Code: "invalid-frame", Message: err.Error()}})
		return
	}
	var request Request
	if err := json.Unmarshal(data, &request); err != nil {
		_ = writeFrame(connection, Response{OK: false, Error: &RPCError{Code: "invalid-request", Message: "request must be valid JSON"}})
		return
	}
	response := s.dispatch(request)
	_ = writeFrame(connection, response)
}

func (s *Server) dispatch(request Request) Response {
	response := Response{ID: request.ID}
	wanted := s.tokenHash
	actual := sha256.Sum256([]byte(request.AuthToken))
	if subtle.ConstantTimeCompare(wanted[:], actual[:]) != 1 {
		response.Error = &RPCError{Code: "unauthorized", Message: "client authentication failed"}
		return response
	}
	s.lastActivity.Store(time.Now().UnixMilli())
	switch request.Method {
	case MethodValidate, MethodApply:
		var params ConfigParams
		if err := strictParams(request.Params, &params); err != nil || !isJSONObject(params.Config) {
			response.Error = &RPCError{Code: "invalid-params", Message: "a structured config object is required"}
			return response
		}
		var err error
		if request.Method == MethodValidate {
			err = s.service.Validate(params.Config)
		} else {
			err = s.service.Apply(params.Config)
		}
		if err != nil {
			response.Error = &RPCError{Code: "operation-failed", Message: err.Error()}
			return response
		}
		response.OK = true
		response.Result = s.service.Status()
	case MethodStop:
		if len(request.Params) != 0 && string(request.Params) != "{}" && string(request.Params) != "null" {
			response.Error = &RPCError{Code: "invalid-params", Message: "stop does not accept parameters"}
			return response
		}
		if err := s.service.Stop(); err != nil {
			response.Error = &RPCError{Code: "operation-failed", Message: err.Error()}
			return response
		}
		response.OK = true
		response.Result = s.service.Status()
	case MethodStatus:
		if !emptyParams(request.Params) {
			response.Error = &RPCError{Code: "invalid-params", Message: "status does not accept parameters"}
			return response
		}
		response.OK = true
		response.Result = s.service.Status()
	case MethodHeartbeat:
		if !emptyParams(request.Params) {
			response.Error = &RPCError{Code: "invalid-params", Message: "heartbeat does not accept parameters"}
			return response
		}
		response.OK = true
		response.Result = Heartbeat{UnixMillis: time.Now().UnixMilli()}
	case MethodShutdown:
		if !emptyParams(request.Params) {
			response.Error = &RPCError{Code: "invalid-params", Message: "shutdown does not accept parameters"}
			return response
		}
		if err := s.service.Stop(); err != nil {
			response.Error = &RPCError{Code: "operation-failed", Message: err.Error()}
			return response
		}
		response.OK = true
		response.Result = s.service.Status()
		s.shutdownOnce.Do(func() { close(s.shutdown) })
	case MethodFaultTerminateCore:
		if !emptyParams(request.Params) {
			response.Error = &RPCError{Code: "invalid-params", Message: "fault termination does not accept parameters"}
			return response
		}
		service, ok := s.service.(FaultService)
		if !ok {
			response.Error = &RPCError{Code: "unknown-method", Message: "method is not allowed"}
			return response
		}
		if err := service.FaultTerminateCore(); err != nil {
			response.Error = &RPCError{Code: "operation-failed", Message: err.Error()}
			return response
		}
		response.OK = true
		response.Result = s.service.Status()
	default:
		response.Error = &RPCError{Code: "unknown-method", Message: "method is not allowed"}
	}
	return response
}

func strictParams(data []byte, destination any) error {
	decoder := json.NewDecoder(bytesReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}

func emptyParams(data []byte) bool {
	return len(data) == 0 || string(data) == "{}" || string(data) == "null"
}

func isJSONObject(data []byte) bool {
	var object map[string]json.RawMessage
	return len(data) > 0 && json.Unmarshal(data, &object) == nil && object != nil
}

func readFrame(reader io.Reader) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(header[:])
	if length == 0 || length > MaxMessageSize {
		return nil, fmt.Errorf("message size %d is outside allowed range", length)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, err
	}
	return data, nil
}

func writeFrame(writer io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) > MaxMessageSize {
		return errors.New("response exceeds message limit")
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(data)))
	if _, err := writer.Write(header[:]); err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

type byteReader struct {
	data   []byte
	offset int
}

func bytesReader(data []byte) *byteReader { return &byteReader{data: data} }
func (r *byteReader) Read(target []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	count := copy(target, r.data[r.offset:])
	r.offset += count
	return count, nil
}
