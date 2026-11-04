package agentbus

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ctxKey int

const botCtxKey ctxKey = iota

// Auth maps bearer tokens to the bot each one speaks for.
type Auth struct {
	creds []credential
}

type credential struct {
	bot   string
	token string
}

// ParseAuth reads credentials from a "bot:token,bot2:token2" string, the shape
// MCP_TOKENS carries. Tokens must be long enough not to be guessed by a peer
// that can reach the port.
func ParseAuth(spec string) (*Auth, error) {
	a := &Auth{}
	seenBot := map[string]bool{}
	seenTok := map[string]bool{}

	for _, pair := range strings.Split(spec, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		bot, token, ok := strings.Cut(pair, ":")
		bot, token = strings.TrimSpace(bot), strings.TrimSpace(token)
		if !ok || bot == "" || token == "" {
			return nil, errors.New("credentials must be formatted as bot:token")
		}
		if !botNameRe.MatchString(bot) {
			return nil, fmt.Errorf("bot name %q must match %s", bot, botNameRe)
		}
		if len(token) < 24 {
			return nil, fmt.Errorf("token for %q is shorter than 24 characters", bot)
		}
		if seenBot[bot] {
			return nil, fmt.Errorf("bot %q listed twice", bot)
		}
		// Two bots sharing a token would make the board's identities
		// meaningless, which is the one property the token is there to provide.
		if seenTok[token] {
			return nil, errors.New("the same token is assigned to more than one bot")
		}
		seenBot[bot], seenTok[token] = true, true
		a.creds = append(a.creds, credential{bot: bot, token: token})
	}

	if len(a.creds) == 0 {
		return nil, errors.New("no credentials configured")
	}
	sort.Slice(a.creds, func(i, j int) bool { return a.creds[i].bot < a.creds[j].bot })
	return a, nil
}

// Bots lists the configured participants. Handy for logging what was loaded
// without printing the tokens themselves.
func (a *Auth) Bots() []string {
	out := make([]string, 0, len(a.creds))
	for _, c := range a.creds {
		out = append(out, c.bot)
	}
	return out
}

// lookup resolves a token to a bot name in constant time with respect to the
// token contents, so a caller cannot recover a valid token byte by byte.
func (a *Auth) lookup(token string) (string, bool) {
	var bot string
	var found bool
	for _, c := range a.creds {
		if subtle.ConstantTimeCompare([]byte(c.token), []byte(token)) == 1 {
			bot, found = c.bot, true
		}
	}
	return bot, found
}

// authenticate resolves the bearer token and puts the caller's bot name in the
// request context.
func (a *Auth) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="agentbus"`)
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		bot, ok := a.lookup(strings.TrimSpace(token))
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="agentbus"`)
			http.Error(w, "unknown token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), botCtxKey, bot)))
	})
}

// Handler returns the authenticated MCP endpoint for the registry.
func Handler(reg *Registry, auth *Auth) http.Handler {
	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		// The identity was established by the middleware; the session can only
		// ever write as that bot.
		bot, _ := r.Context().Value(botCtxKey).(string)
		return NewServer(reg, bot)
	}, &mcp.StreamableHTTPOptions{Stateless: true})

	return auth.authenticate(mcpHandler)
}
