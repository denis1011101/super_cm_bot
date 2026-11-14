package agentbus

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "super-cm-bot-agentbus"
	serverVersion = "0.1.0"
)

// reportInput is what a bot publishes about itself. There is deliberately no
// "bot" field: the identity comes from the caller's token, so a participant
// cannot report on behalf of a peer.
type reportInput struct {
	Health     string `json:"health" jsonschema:"how the bot is running: ok, degraded or down"`
	Quota      string `json:"quota" jsonschema:"LLM budget state: ok, exhausted or unknown"`
	ErrorClass string `json:"error_class,omitempty" jsonschema:"category of the last failure: RESOURCE_EXHAUSTED, UPSTREAM_UNAVAILABLE, TIMEOUT, MISCONFIGURED or OTHER"`
	Detail     string `json:"detail,omitempty" jsonschema:"short human note; never include secrets, URLs or raw error text"`
}

type getInput struct {
	Bot string `json:"bot" jsonschema:"name of the bot to look up"`
}

type listOutput struct {
	Bots []Status `json:"bots"`
}

// NewServer builds an MCP server whose write tool is bound to callerBot. Pass
// an empty callerBot for a read-only session: it can observe the board but not
// post to it.
func NewServer(reg *Registry, callerBot string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)

	if callerBot != "" {
		mcp.AddTool(s, &mcp.Tool{
			Name: "report_status",
			Description: "Publish your own status to the shared board so other bots can see it. " +
				"Call it when your LLM quota runs out, when calls start failing, and again when you recover. " +
				"Your bot identity is taken from your credentials, not from the arguments.",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, in reportInput) (*mcp.CallToolResult, Status, error) {
			st, err := reg.Report(callerBot, Health(in.Health), Quota(in.Quota), in.ErrorClass, in.Detail)
			if err != nil {
				return nil, Status{}, err
			}
			return nil, st, nil
		})
	}

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_status",
		Description: "Read one bot's last published status. A status marked stale means that bot " +
			"stopped reporting, which is not the same as it reporting that it is healthy.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getInput) (*mcp.CallToolResult, Status, error) {
		st, ok := reg.Get(in.Bot)
		if !ok {
			return nil, Status{}, fmt.Errorf("no status reported by %q", in.Bot)
		}
		return nil, st, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_status",
		Description: "Read the last published status of every bot on the board.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listOutput, error) {
		return nil, listOutput{Bots: reg.List()}, nil
	})

	return s
}
