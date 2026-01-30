package policy

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// DynamicRule represents a user-defined policy rule.
type DynamicRule struct {
	ID          string   `json:"id"`
	Condition   string   `json:"condition"`    // CEL expression: "cost > 100 && tags.env == 'prod'"
	Action      string   `json:"action"`       // "block", "warn", "approve"
	Priority    int      `json:"priority"`     // Higher wins
	TargetKinds []string `json:"target_kinds"` // Efficient filtering (e.g. ["AWS::S3::Bucket"])
}

// CELEngine handles dynamic rule execution.
type CELEngine struct {
	env               *cel.Env
	programs          map[string]cel.Program
	rules             map[string]DynamicRule
	index             map[string][]string // Kind -> []RuleID
	violationsCounter metric.Int64Counter
}

// EvaluationContext defines CEL rule input data.
type EvaluationContext struct {
	ID    string                 `cel:"id"`
	Kind  string                 `cel:"kind"`
	Cost  float64                `cel:"cost"`
	Tags  map[string]string      `cel:"tags"`
	Props map[string]interface{} `cel:"props"`
}

// NewCELEngine initializes the CEL environment.
func NewCELEngine() (*CELEngine, error) {
	// Register EvaluationContext.
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

	// Initialize metrics.
	meter := otel.Meter("cloudslash/policy")
	violations, err := meter.Int64Counter("policy_violations_total", metric.WithDescription("Total number of policy violations detected"))
	if err != nil {
		// Log initialization failure.
		slog.Warn("Failed to initialize policy metric", "error", err)
	}

	return &CELEngine{
		env:               env,
		programs:          make(map[string]cel.Program),
		rules:             make(map[string]DynamicRule),
		index:             make(map[string][]string),
		violationsCounter: violations,
	}, nil
}

// Compile prepares rules for execution.
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
		e.rules[r.ID] = r

		// Handle global rules.
		if len(r.TargetKinds) == 0 {
			e.index["*"] = append(e.index["*"], r.ID)
		} else {
			for _, kind := range r.TargetKinds {
				if kind == "*" {
					e.index["*"] = append(e.index["*"], r.ID)
				} else {
					e.index[kind] = append(e.index[kind], r.ID)
				}
			}
		}
	}
	return nil
}

// Evaluate validates input against compiled rules.
// Uses inverted index for O(1) lookup.
// Returns sorted matches.
func (e *CELEngine) Evaluate(ctx context.Context, evalCtx EvaluationContext) ([]DynamicRule, error) {
	var matches []DynamicRule

	// Map variables.
	vars := map[string]interface{}{
		"id":    evalCtx.ID,
		"kind":  evalCtx.Kind,
		"cost":  evalCtx.Cost,
		"tags":  evalCtx.Tags,
		"props": evalCtx.Props,
	}

	// Fetch candidate rules.
	// Rules targeting this specific Kind + Global Rules
	candidates := make([]string, 0, len(e.index[evalCtx.Kind])+len(e.index["*"]))
	candidates = append(candidates, e.index[evalCtx.Kind]...)
	candidates = append(candidates, e.index["*"]...)

	// Dedup is only needed if a rule is in both...
	// For performance, we assume user is sane.
	
	evaluated := make(map[string]bool)

	// Evaluate candidates (O(k)).
	for _, id := range candidates {
		if evaluated[id] {
			continue // Skip duplicates.
		}
		evaluated[id] = true

		prg, ok := e.programs[id]
		if !ok {
			continue
		}

		out, _, err := prg.Eval(vars)
		if err != nil {
			slog.Error("Rule evaluation failed", "rule_id", id, "error", err)
			continue
		}

		if match, ok := out.Value().(bool); ok && match {
			if rule, exists := e.rules[id]; exists {
				matches = append(matches, rule)
				
				// Record violation metric.
				if e.violationsCounter != nil {
					e.violationsCounter.Add(ctx, 1, metric.WithAttributes(
						attribute.String("rule_id", id),
						attribute.String("resource_type", evalCtx.Kind),
					))
				}
			}
		}
	}
	
	// Sort by priority.
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Priority != matches[j].Priority {
			return matches[i].Priority > matches[j].Priority
		}
		return matches[i].ID < matches[j].ID // Stable sort.
	})

	return matches, nil
}
