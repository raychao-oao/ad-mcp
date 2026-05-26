package policy

import (
	"context"
	"fmt"

	mcppolicy "github.com/raychao-oao/mcp-policy/pkg/policy"
	"github.com/raychao-oao/mcp-policy/pkg/yamlengine"
)

// Authorize checks whether the tool call is permitted by the policy engine.
// engine may be nil when no policy file is configured — open access.
// approvalToken is empty for read-only tools; MVP2+ write tools pass the
// cred-mcp approval token here for RequireApproval decisions.
func Authorize(ctx context.Context, engine *yamlengine.Engine, tool string, resource mcppolicy.Resource, approvalToken string) error {
	if engine == nil {
		return nil
	}
	req := mcppolicy.PolicyRequest{
		Actor:    mcppolicy.Actor{ID: "anonymous"},
		Consumer: mcppolicy.Consumer{ID: "claude-code", Name: "Claude Code"},
		Tool:     tool,
		Resource: resource,
	}
	decision, err := engine.Authorize(ctx, req)
	if err != nil {
		return fmt.Errorf("policy error: %w", err)
	}
	switch decision.Decision {
	case mcppolicy.Allow:
		return nil
	case mcppolicy.RequireApproval:
		if approvalToken != "" {
			// Token validation will be implemented in MVP2.
			return nil
		}
		return fmt.Errorf("approval required (%s): %s", decision.Risk, decision.Reason)
	default:
		return fmt.Errorf("denied: %s", decision.Reason)
	}
}
