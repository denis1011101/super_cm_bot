package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/denis1011101/super_cm_bot/app/agentbus"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const testToken = "0123456789abcdef0123456789abcdef"

func TestRegistryRejectsUnknownEnums(t *testing.T) {
	reg := agentbus.NewRegistry(time.Minute)

	cases := []struct {
		name          string
		bot, errClass string
		health        agentbus.Health
		quota         agentbus.Quota
	}{
		{name: "bad bot name", bot: "not a bot", health: agentbus.HealthOK, quota: agentbus.QuotaOK},
		{name: "empty bot name", bot: "", health: agentbus.HealthOK, quota: agentbus.QuotaOK},
		{name: "unknown health", bot: "peer", health: "fine", quota: agentbus.QuotaOK},
		{name: "unknown quota", bot: "peer", health: agentbus.HealthOK, quota: "empty"},
		{name: "unknown error class", bot: "peer", health: agentbus.HealthOK, quota: agentbus.QuotaOK, errClass: "KABOOM"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := reg.Report(tc.bot, tc.health, tc.quota, tc.errClass, ""); !errors.Is(err, agentbus.ErrInvalid) {
				t.Fatalf("expected ErrInvalid, got %v", err)
			}
		})
	}

	if got := reg.List(); len(got) != 0 {
		t.Fatalf("rejected reports must not be stored, got %d entries", len(got))
	}
}

func TestRegistryRedactsCredentials(t *testing.T) {
	reg := agentbus.NewRegistry(time.Minute)

	// This is the exact shape gemini.go builds: the key rides in the URL, so an
	// error string pasted into a detail would publish it to every peer.
	detail := "Post \"https://generativelanguage.googleapis.com/v1beta/models/x:generateContent?key=AIzaSyFAKEKEY1234567890abcd\": timeout"
	st, err := reg.Report("peer", agentbus.HealthDegraded, agentbus.QuotaUnknown, agentbus.ErrTimeout, detail)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if strings.Contains(st.Detail, "AIzaSyFAKEKEY1234567890abcd") {
		t.Fatalf("api key survived redaction: %q", st.Detail)
	}
	if !strings.Contains(st.Detail, "<redacted>") {
		t.Fatalf("expected a redaction marker, got %q", st.Detail)
	}

	st, err = reg.Report("peer", agentbus.HealthDegraded, agentbus.QuotaUnknown, agentbus.ErrOther, "Authorization: Bearer sk-secret-value")
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if strings.Contains(st.Detail, "sk-secret-value") {
		t.Fatalf("bearer token survived redaction: %q", st.Detail)
	}
}

func TestRegistryTruncatesWithoutSplittingRunes(t *testing.T) {
	reg := agentbus.NewRegistry(time.Minute)
	st, err := reg.Report("peer", agentbus.HealthOK, agentbus.QuotaOK, agentbus.ErrNone, strings.Repeat("я", 500))
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if !strings.HasSuffix(st.Detail, "…") {
		t.Fatalf("expected truncation, got %q", st.Detail)
	}
	if strings.ContainsRune(st.Detail, '�') {
		t.Fatal("truncation split a multi-byte rune")
	}
}

func TestRegistryMarksSilentPeerStaleAndDropsItsQuota(t *testing.T) {
	reg := agentbus.NewRegistry(50 * time.Millisecond)
	if _, err := reg.Report("peer", agentbus.HealthOK, agentbus.QuotaOK, agentbus.ErrNone, ""); err != nil {
		t.Fatalf("Report: %v", err)
	}

	if st, ok := reg.Get("peer"); !ok || st.Stale || st.Quota != agentbus.QuotaOK {
		t.Fatalf("fresh entry should be usable, got %+v ok=%v", st, ok)
	}

	time.Sleep(80 * time.Millisecond)

	st, ok := reg.Get("peer")
	if !ok {
		t.Fatal("stale entry should still be listed")
	}
	if !st.Stale {
		t.Fatal("entry past its TTL must be marked stale")
	}
	// A peer that died while healthy must not keep advertising a working quota.
	if st.Quota != agentbus.QuotaUnknown {
		t.Fatalf("stale quota should read unknown, got %q", st.Quota)
	}
}

func TestParseAuthRejectsUnusableCredentials(t *testing.T) {
	cases := map[string]string{
		"empty":         "",
		"missing token": "peer",
		"empty token":   "peer:",
		"short token":   "peer:tooshort",
		"bad bot name":  "not a bot:" + testToken,
		"duplicate bot": "peer:" + testToken + ",peer:ffffffffffffffffffffffffffff",
		"shared token":  "peer:" + testToken + ",other:" + testToken,
	}

	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := agentbus.ParseAuth(spec); err == nil {
				t.Fatalf("expected %q to be rejected", spec)
			}
		})
	}

	auth, err := agentbus.ParseAuth("peer:" + testToken + ", other:ffffffffffffffffffffffffffff")
	if err != nil {
		t.Fatalf("valid spec rejected: %v", err)
	}
	if got := auth.Bots(); len(got) != 2 || got[0] != "other" || got[1] != "peer" {
		t.Fatalf("unexpected bots %v", got)
	}
}

// contentText flattens a tool result's content for assertion messages.
func contentText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// decodeStructured re-marshals a tool's structured output into a typed value,
// which is how a peer bot would consume it.
func decodeStructured(res *mcp.CallToolResult, out any) error {
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// tokenTransport attaches a bearer token the way a peer bot's MCP client would.
type tokenTransport struct {
	token string
	base  http.RoundTripper
}

func (t tokenTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	if t.token != "" {
		clone.Header.Set("Authorization", "Bearer "+t.token)
	}
	return t.base.RoundTrip(clone)
}

func newBusServer(t *testing.T, reg *agentbus.Registry, spec string) *httptest.Server {
	t.Helper()
	auth, err := agentbus.ParseAuth(spec)
	if err != nil {
		t.Fatalf("ParseAuth: %v", err)
	}
	srv := httptest.NewServer(agentbus.Handler(reg, auth))
	t.Cleanup(srv.Close)
	return srv
}

func connect(t *testing.T, endpoint, token string) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-peer", Version: "0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             endpoint,
		HTTPClient:           &http.Client{Transport: tokenTransport{token: token, base: http.DefaultTransport}},
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestBusRejectsMissingAndUnknownTokens(t *testing.T) {
	reg := agentbus.NewRegistry(time.Minute)
	srv := newBusServer(t, reg, "peer:"+testToken)

	for name, token := range map[string]string{
		"no token":    "",
		"wrong token": "ffffffffffffffffffffffffffffffff",
	} {
		t.Run(name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", resp.StatusCode)
			}
		})
	}
}

func TestBusIdentityComesFromTokenNotArguments(t *testing.T) {
	reg := agentbus.NewRegistry(time.Minute)
	srv := newBusServer(t, reg, "peer:"+testToken+",victim:ffffffffffffffffffffffffffff")
	session := connect(t, srv.URL, testToken)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// "peer" tries to publish as "victim" by smuggling the name into the
	// arguments. The tool takes no such field, and the schema rejects it
	// outright rather than quietly ignoring it.
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "report_status",
		Arguments: map[string]any{
			"health": "down",
			"quota":  "exhausted",
			"bot":    "victim",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatal("a smuggled bot field should have been rejected")
	}

	// The legitimate call lands under the authenticated identity, and nothing
	// at all is recorded against the peer it tried to impersonate.
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "report_status",
		Arguments: map[string]any{"health": "down", "quota": "exhausted"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("report_status failed: %s", contentText(res))
	}

	if _, ok := reg.Get("victim"); ok {
		t.Fatal("a bot managed to publish under another bot's name")
	}
	st, ok := reg.Get("peer")
	if !ok {
		t.Fatal("report was not stored under the authenticated identity")
	}
	if st.Health != agentbus.HealthDown || st.Quota != agentbus.QuotaExhausted {
		t.Fatalf("unexpected stored status %+v", st)
	}
}

func TestBusPeerSeesExhaustedQuotaOfAnother(t *testing.T) {
	reg := agentbus.NewRegistry(time.Minute)
	srv := newBusServer(t, reg, "peer:"+testToken+",reader:ffffffffffffffffffffffffffff")

	// This is the whole point of the board: the bot that ran out of budget says
	// so once, and the other one finds out without a Telegram message.
	if _, err := reg.Report("super_cum_lovers_bot", agentbus.HealthDegraded, agentbus.QuotaExhausted, agentbus.ErrResourceExausted, ""); err != nil {
		t.Fatalf("Report: %v", err)
	}

	session := connect(t, srv.URL, "ffffffffffffffffffffffffffff")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_status",
		Arguments: map[string]any{"bot": "super_cum_lovers_bot"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("get_status failed: %+v", res.Content)
	}

	var got agentbus.Status
	if err := decodeStructured(res, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Quota != agentbus.QuotaExhausted || got.ErrorClass != agentbus.ErrResourceExausted {
		t.Fatalf("peer did not learn about the exhausted quota: %+v", got)
	}
}

func TestBusReportsUnknownBotAsToolError(t *testing.T) {
	reg := agentbus.NewRegistry(time.Minute)
	srv := newBusServer(t, reg, "peer:"+testToken)
	session := connect(t, srv.URL, testToken)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_status",
		Arguments: map[string]any{"bot": "nobody"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected a tool error for an unknown bot")
	}
}
