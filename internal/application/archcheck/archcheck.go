// Package archcheck runs the hexagonal + SOLID architecture rules
// (specs/HEXAGONAL_ARCHITECTURE.md, specs/SOLID_GO.md) over a Go module.
package archcheck

import (
	"context"

	"github.com/insurgencygames/ai-loops/internal/domain/archrule"
)

// Parser turns a module tree into the package model the rules evaluate.
type Parser interface {
	Parse(ctx context.Context, root string) (archrule.Module, error)
}

// Service runs the checks.
type Service struct{ parser Parser }

// New builds a Service.
func New(p Parser) *Service { return &Service{parser: p} }

// Check parses root and evaluates every rule with the given layer layout.
func (s *Service) Check(ctx context.Context, root string, layers archrule.Layers) (archrule.Report, error) {
	m, err := s.parser.Parse(ctx, root)
	if err != nil {
		return archrule.Report{}, err
	}
	return archrule.Evaluate(m, layers), nil
}
