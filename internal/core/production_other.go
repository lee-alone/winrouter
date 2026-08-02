//go:build !windows

package core

import "errors"

type ProductionOptions struct {
	CorePath       string
	StateDirectory string
	Restart        RestartPolicy
}

func NewProductionController(ProductionOptions) (*Controller, error) {
	return nil, errors.New("production core controller is supported only on Windows")
}
