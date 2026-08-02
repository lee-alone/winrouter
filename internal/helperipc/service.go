package helperipc

import (
	"context"
	"errors"
	"time"

	"winrouter/internal/config"
	"winrouter/internal/core"
)

type ControllerService struct {
	Controller            *core.Controller
	Timeout               time.Duration
	AllowFaultTermination bool
}

func (s ControllerService) timeout() time.Duration {
	if s.Timeout <= 0 {
		return 30 * time.Second
	}
	return s.Timeout
}
func (s ControllerService) generated(data []byte) ([]byte, error) {
	input, err := config.DecodeMVPConfig(data)
	if err != nil {
		return nil, err
	}
	generated, err := config.GenerateMVP(input)
	if err != nil {
		return nil, err
	}
	return generated.JSON, nil
}
func (s ControllerService) Validate(data []byte) error {
	generated, err := s.generated(data)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout())
	defer cancel()
	return s.Controller.Validate(ctx, generated)
}
func (s ControllerService) Apply(data []byte) error {
	generated, err := s.generated(data)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout())
	defer cancel()
	return s.Controller.Apply(ctx, generated)
}
func (s ControllerService) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout())
	defer cancel()
	return s.Controller.Stop(ctx)
}
func (s ControllerService) Status() core.Status { return s.Controller.Status() }
func (s ControllerService) FaultTerminateCore() error {
	if !s.AllowFaultTermination {
		return errors.New("core fault injection is disabled")
	}
	return s.Controller.FaultTerminateCore()
}
