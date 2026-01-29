package policy

import (
	"fmt"
	"log/slog"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
)

// DynamicRule represents a user-defined policy rule (e.g. from YAML).
type DynamicRule struct {
	ID        string `json:"id"`
	Condition string `json:"condition"` // CEL expression: "cost > 100 && tags.env == 'prod'"
	Action    string `json:"action"`    // "block", "warn", "approve"
}

// CELEngine manages the compilation and execution of dynamic rules.
type CELEngine struct {
	env      *cel.Env
	programs map[string]cel.Program
}

// NewCELEngine initializes the CEL environment with standard variable declarations.
func NewCELEngine() (*CELEngine, error) {
	// Initialize the CEL environment with the supported variable declarations.
	env, err := cel.NewEnv(
		cel.Declarations(
			decls.NewVar("id", decls.String),
			decls.NewVar("kind", decls.String),
			decls.NewVar("cost", decls.Double),
			decls.NewVar("tags", decls.NewMapType(decls.String, decls.String)),
			decls.NewVar("props", decls.NewMapType(decls.String, decls.Dyn)),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL env: %w", err)
	}

	return &CELEngine{
		env:      env,
		programs: make(map[string]cel.Program),
	}, nil
}

// Compile compiles a list of rules into executable programs.
func (e *CELEngine) Compile(rules []DynamicRule) error {
	for _, r := range rules {
		ast, issues := e.env.Compile(r.Condition)
		if issues != nil && issues.Err() != nil {
			return fmt.Errorf("rule %s compilation error: %w", r.ID, issues.Err())
		}

		prg, err := e.env.Program(ast)
		if err != nil {
			return fmt.Errorf("rule %s program creation error: %w", r.ID, err)
		}

		e.programs[r.ID] = prg
	}
	return nil
}

// Evaluate checks if the input matches any rules.
// Returns a list of matched Rule IDs.
func (e *CELEngine) Evaluate(data map[string]interface{}) ([]string, error) {
	var matches []string

	// Flatten data into top-level variables
	vars := data

	for id, prg := range e.programs {
		out, _, err := prg.Eval(vars)
		if err != nil {
			slog.Error("Rule evaluation failed", "rule_id", id, "error", err)
			continue
		}

		// We expect rules to return a boolean (true = match/violation)
		if match, ok := out.Value().(bool); ok && match {
			matches = append(matches, id)
		}
	}

	return matches, nil
}
