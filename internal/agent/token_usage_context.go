package agent

import (
	"context"

	"github.com/fisk086/aiops/internal/schema"
	einoschema "github.com/cloudwego/eino/schema"
)

// usageCtxKey is the context key for per-request token accounting (facade / non-invasive).
type usageCtxKey struct{}

// usageSession is created per chat completion (stream or non-stream).
type usageSession struct {
	acc         *usageAccumulator
	agent       *schema.AgentWithRuntime
	auditUserID string
	modelName   string
}

// withUsageTracking attaches a fresh usage session to ctx. Call flushUsageSession when the request ends.
func (r *Runtime) withUsageTracking(ctx context.Context, agent *schema.AgentWithRuntime, auditUserID string) context.Context {
	if agent == nil {
		return ctx
	}
	s := &usageSession{
		acc:         &usageAccumulator{},
		agent:       agent,
		auditUserID: auditUserID,
	}
	return context.WithValue(ctx, usageCtxKey{}, s)
}

// ensureUsageTracking attaches a session only if ctx does not already carry one (avoids nested overwrites).
func (r *Runtime) ensureUsageTracking(ctx context.Context, agent *schema.AgentWithRuntime, auditUserID string) context.Context {
	if _, ok := ctx.Value(usageCtxKey{}).(*usageSession); ok {
		return ctx
	}
	return r.withUsageTracking(ctx, agent, auditUserID)
}

func usageAccumulateFromContext(ctx context.Context, msg *einoschema.Message) {
	s, _ := ctx.Value(usageCtxKey{}).(*usageSession)
	if s == nil || s.acc == nil {
		return
	}
	s.acc.addMessage(msg)
}

func setUsageModelName(ctx context.Context, modelName string) {
	s, _ := ctx.Value(usageCtxKey{}).(*usageSession)
	if s != nil && modelName != "" {
		s.modelName = modelName
	}
}

// flushUsageSession persists aggregated tokens for this request (no-op if no session or sink).
func (r *Runtime) flushUsageSession(ctx context.Context) {
	s, _ := ctx.Value(usageCtxKey{}).(*usageSession)
	if s == nil {
		return
	}
	r.flushTokenUsage(ctx, s.agent, s.auditUserID, s.acc)
}
