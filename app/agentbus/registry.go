// Package agentbus is the shared status board that bots publish to and read
// from over MCP, instead of trying to talk to each other through the Telegram
// chat. Telegram only delivers bot-to-bot messages under a narrow set of
// conditions, and it drops them silently when those are not met, so it is not a
// channel a bot can rely on to learn that a peer is out of quota.
package agentbus

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Health is how well a bot believes it is running.
type Health string

// Quota is what a bot knows about its own LLM budget.
type Quota string

const (
	HealthOK       Health = "ok"
	HealthDegraded Health = "degraded"
	HealthDown     Health = "down"

	QuotaOK        Quota = "ok"
	QuotaExhausted Quota = "exhausted"
	QuotaUnknown   Quota = "unknown"
)

// ErrorClass values are a closed set on purpose. Free-form error text is how
// secrets leak: this bot puts its API key in the request URL, so a transport
// error string can carry the key with it. Peers get a category, never a
// message the runtime built.
const (
	ErrNone             = ""
	ErrResourceExausted = "RESOURCE_EXHAUSTED"
	ErrUpstreamDown     = "UPSTREAM_UNAVAILABLE"
	ErrTimeout          = "TIMEOUT"
	ErrConfig           = "MISCONFIGURED"
	ErrOther            = "OTHER"
)

const (
	maxDetailLen = 200
	maxBotLen    = 64
)

var (
	validHealth = map[Health]bool{HealthOK: true, HealthDegraded: true, HealthDown: true}
	validQuota  = map[Quota]bool{QuotaOK: true, QuotaExhausted: true, QuotaUnknown: true}
	validErr    = map[string]bool{
		ErrNone: true, ErrResourceExausted: true, ErrUpstreamDown: true,
		ErrTimeout: true, ErrConfig: true, ErrOther: true,
	}
	botNameRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
)

// ErrInvalid is returned for any report the registry refuses to store.
var ErrInvalid = errors.New("invalid status report")

// Status is one bot's view of itself, as published to its peers.
type Status struct {
	Bot        string    `json:"bot"`
	Health     Health    `json:"health"`
	Quota      Quota     `json:"quota"`
	ErrorClass string    `json:"error_class,omitempty"`
	Detail     string    `json:"detail,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
	// Stale marks a bot that stopped reporting. A peer that went silent is not
	// the same as a peer that reported being healthy, and a reader that cannot
	// tell them apart will keep trusting a status frozen at the moment a
	// process died.
	Stale bool `json:"stale"`
}

// Registry holds the last status each bot published.
type Registry struct {
	mu      sync.RWMutex
	ttl     time.Duration
	now     func() time.Time
	entries map[string]Status
}

// NewRegistry returns a registry that marks entries stale after ttl.
func NewRegistry(ttl time.Duration) *Registry {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &Registry{ttl: ttl, now: time.Now, entries: make(map[string]Status)}
}

// Report stores a bot's status. bot is the caller's authenticated identity, not
// a value it chose in the payload, so no participant can post as another.
func (r *Registry) Report(bot string, health Health, quota Quota, errClass, detail string) (Status, error) {
	if !botNameRe.MatchString(bot) {
		return Status{}, fmt.Errorf("%w: bot name must match %s", ErrInvalid, botNameRe)
	}
	if !validHealth[health] {
		return Status{}, fmt.Errorf("%w: unknown health %q", ErrInvalid, health)
	}
	if !validQuota[quota] {
		return Status{}, fmt.Errorf("%w: unknown quota %q", ErrInvalid, quota)
	}
	if !validErr[errClass] {
		return Status{}, fmt.Errorf("%w: unknown error_class %q", ErrInvalid, errClass)
	}

	s := Status{
		Bot:        bot,
		Health:     health,
		Quota:      quota,
		ErrorClass: errClass,
		Detail:     Redact(detail),
		UpdatedAt:  r.now().UTC(),
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[bot] = s
	return s, nil
}

// Get returns one bot's status, with staleness resolved as of now.
func (r *Registry) Get(bot string) (Status, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.entries[bot]
	if !ok {
		return Status{}, false
	}
	return r.withStaleness(s), true
}

// List returns every known bot, ordered by name so callers get a stable view.
func (r *Registry) List() []Status {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Status, 0, len(r.entries))
	for _, s := range r.entries {
		out = append(out, r.withStaleness(s))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Bot < out[j].Bot })
	return out
}

// withStaleness must be called with r.mu held.
func (r *Registry) withStaleness(s Status) Status {
	if r.now().Sub(s.UpdatedAt) > r.ttl {
		s.Stale = true
		// A stale entry says nothing current about the budget. Reporting the
		// last known value as if it still held is how a reader concludes a dead
		// peer is fine.
		s.Quota = QuotaUnknown
	}
	return s
}

var (
	keyInURLRe = regexp.MustCompile(`(?i)([?&](?:key|api_?key|access_token|token)=)[^&\s]+`)
	bearerRe   = regexp.MustCompile(`(?i)(bearer\s+)\S+`)
	// Google API keys are a recognisable shape; catch them even outside a URL.
	googleKeyRe = regexp.MustCompile(`AIza[0-9A-Za-z_\-]{10,}`)
)

// Redact strips credentials from text before it is stored or shown to a peer,
// then truncates it. It is a backstop, not a licence to pass raw error strings
// through: callers should send a classified category instead.
func Redact(s string) string {
	s = strings.TrimSpace(s)
	s = keyInURLRe.ReplaceAllString(s, "${1}<redacted>")
	s = bearerRe.ReplaceAllString(s, "${1}<redacted>")
	s = googleKeyRe.ReplaceAllString(s, "<redacted>")
	// Count runes, not bytes: details arrive in Russian as often as not, and a
	// byte slice would cut a multi-byte rune in half.
	if r := []rune(s); len(r) > maxDetailLen {
		s = string(r[:maxDetailLen]) + "…"
	}
	return s
}
