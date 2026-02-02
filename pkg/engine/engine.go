package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"errors"

	internalconfig "github.com/DrSkyle/cloudslash/v2/pkg/config"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/history"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/notifier"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/pricing"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/swarm"
	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	"github.com/DrSkyle/cloudslash/v2/pkg/telemetry"
	"github.com/DrSkyle/cloudslash/v2/pkg/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ErrPartialResult indicates the scan completed but some resources were skipped due to API errors.
var ErrPartialResult = errors.New("scan completed with partial results")

// Config holds engine settings.
type Config struct {
	Region           string
	TFStatePath      string
	MockMode         bool
	AllProfiles      bool
	RequiredTags     string
	SlackWebhook     string
	SlackChannel     string
	Headless         bool
	DisableCWMetrics bool
	Verbose          bool
	MaxConcurrency   int
	JsonLogs         bool
	RulesFile        string
	HistoryURL       string // "s3://bucket/key" or empty for local
	OutputDir        string // Directory for generated artifacts
	Heuristics       internalconfig.HeuristicConfig

	// StrictMode forces a non-zero exit code on partial failures.
	StrictMode bool

	// Pricing overrides.
	DiscountRate float64 // Manual EDP/RI rate (e.g. 0.82)

	// Telemetry config.
	OtelEndpoint  string // "http://localhost:4318" or via env
	SkipTelemetry bool   // Set true if embedding in an app that already has OTEL

	// Dependencies.
	Logger   *slog.Logger
	CacheDir string
}

// Engine is the runtime core.
type Engine struct {
	// Core components.
	Graph  *graph.Graph
	Swarm  *swarm.Engine
	Logger *slog.Logger
	Tracer trace.Tracer

	// Immutable config.
	config    Config
	outputDir string
	s3Target  string // "s3://bucket/key" or empty

	// External dependencies.
	History  *history.Client
	Notifier *notifier.SlackClient
	Pricing  *pricing.Client

	// Runtime state.
	doneChan chan struct{}
}

// Option defines a functional configuration override.
type Option func(*Engine)

// New initializes the Engine.
func New(ctx context.Context, opts ...Option) (*Engine, error) {
	// Safe defaults.
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: redactSensitiveData,
	})
	e := &Engine{
		Graph:     graph.NewGraph(),
		Swarm:     swarm.NewEngine(),
		Logger:    slog.New(handler),
		Tracer:    otel.Tracer("cloudslash/engine"),
		outputDir: "cloudslash-out",
	}

	// Apply options.
	for _, opt := range opts {
		opt(e)
	}

	slog.SetDefault(e.Logger)

	// Initialize telemetry.
	if !e.config.SkipTelemetry {
		shutdown, err := telemetry.Init(ctx, version.AppName, version.Current, e.config.OtelEndpoint)
		if err != nil {
			e.Logger.Warn("Telemetry failed", "error", err)
		} else {
			_ = shutdown
		}
	}

	// Initialize history.
	var backend history.Backend
	if e.config.HistoryURL != "" {
		backend = history.NewLocalBackend(".cloudslash/history")
	} else {
		backend = history.NewLocalBackend(".cloudslash/history")
	}

	e.History = history.NewClient(backend)

	return e, nil
}

// WithLogger sets the logger.
func WithLogger(l *slog.Logger) Option {
	return func(e *Engine) {
		e.Logger = l
	}
}

// WithConcurrency sets the swarm limit.
func WithConcurrency(n int) Option {
	return func(e *Engine) {
		if n > 0 {
			e.Swarm.MaxWorkers = n
		}
	}
}

// WithPricing sets pricing provider.
func WithPricing(p *pricing.Client) Option {
	return func(e *Engine) {
		e.Pricing = p
	}
}

// WithConfig sets raw config.
func WithConfig(cfg Config) Option {
	return func(e *Engine) {
		e.config = cfg
		if cfg.OutputDir != "" {
			if strings.HasPrefix(cfg.OutputDir, "s3://") {
				e.s3Target = cfg.OutputDir
				e.outputDir = "cloudslash-out" // Generate locally first
			} else {
				e.outputDir = cfg.OutputDir
			}
		}
		if cfg.Logger != nil {
			e.Logger = cfg.Logger
		}
		if cfg.MaxConcurrency > 0 {
			e.Swarm.MaxWorkers = cfg.MaxConcurrency
		}
	}
}

// Run starts the analysis.
func (e *Engine) Run(ctx context.Context) (bool, *graph.Graph, *swarm.Engine, error) {
	ctx, span := e.Tracer.Start(ctx, "Engine.Run")
	defer span.End()

	// Crash safety.
	defer e.recoverPanic(ctx)

	if !e.config.Headless && !e.config.JsonLogs {
		fmt.Printf("%s %s [%s]\n", version.AppName, version.Current, version.License)
	}

	e.Logger.Info("Starting CloudSlash Engine", "concurrency", e.Swarm.MaxWorkers)
	e.Swarm.Start(ctx)
	defer e.Swarm.Stop()

	// Execute strategy.
	if e.config.MockMode {
		runMockMode(ctx, e)
		return true, e.Graph, e.Swarm, nil
	}

	// Real mode.
	done := runRealPipeline(ctx, e)

	// Await completion.
	if done != nil {
		<-done
	}

	// --- FAANG PATTERN: State Inspection ---
	e.Graph.Mu.RLock()
	isPartial := e.Graph.Metadata.Partial
	failureCount := len(e.Graph.Metadata.FailedScopes)
	e.Graph.Mu.RUnlock()

	if isPartial {
		// 1. Add Telemetry Event (Observability)
		span.SetAttributes(attribute.Bool("scan.partial", true))
		span.SetAttributes(attribute.Int("scan.failed_scopes", failureCount))

		// 2. Enforce Strictness
		if e.config.StrictMode {
			e.Logger.Error("Strict Mode: Failing due to partial scan results")
			return true, e.Graph, e.Swarm, ErrPartialResult
		}

		// If not strict, we log but return nil (success)
		e.Logger.Warn("Scan finished with partial errors (StrictMode=false)")
	}

	return true, e.Graph, e.Swarm, nil
}

// recoverPanic handles failures.
func (e *Engine) recoverPanic(ctx context.Context) {
	if r := recover(); r != nil {
		tr := otel.Tracer("cloudslash/engine")
		// Use independent context.
		_, span := tr.Start(ctx, "CriticalPanic")

		stack := debug.Stack()

		// Record Exception in OTEL
		span.RecordError(fmt.Errorf("%v", r), trace.WithStackTrace(true))
		span.SetStatus(codes.Error, "CRITICAL FAILURE")
		span.SetAttributes(
			attribute.String("crash.stack", string(stack)),
			attribute.String("crash.reason", fmt.Sprintf("%v", r)),
		)
		span.End()

		// Log to Stdout (Container/Serverless friendly)
		e.Logger.Error("CRITICAL FAILURE", "error", r, "stack", string(stack))
		
		// Note: We do not os.Exit(1) here to allow the caller to handle the error state 
		// if CloudSlash is used as a library.
	}
}

// redactSensitiveData scrubs sensitive keys from logs.
func redactSensitiveData(groups []string, a slog.Attr) slog.Attr {
	// List of keys to redact
	sensitiveKeys := map[string]bool{
		"account": true, "password": true, "access_key": true, "token": true,
		"secret": true, "api_key": true, "private_key": true, "auth_token": true,
		"refresh_token": true, "certificate": true, "signature": true,
		"credential": true, "ssh_key": true, "connection_string": true,
	}

	if sensitiveKeys[a.Key] {
		return slog.Attr{
			Key:   a.Key,
			Value: slog.StringValue("[REDACTED]"),
		}
	}
	return a
}
