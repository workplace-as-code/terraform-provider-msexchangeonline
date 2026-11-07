package client

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

// scopeCredential answers GetToken with one token per scope and records the
// scopes it was asked for.
type scopeCredential struct {
	expires map[string]time.Time
	fail    string // scope whose request fails
	scopes  []string
}

func (c *scopeCredential) GetToken(_ context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	scope := opts.Scopes[0]
	c.scopes = append(c.scopes, scope)
	if scope == c.fail {
		return azcore.AccessToken{}, errors.New("denied")
	}
	return azcore.AccessToken{Token: "at-" + scope, ExpiresOn: c.expires[scope]}, nil
}

// The bridge gets an Exchange Online and a Microsoft Graph token; the worker
// is recycled on the earlier expiry, so ExpiresOn is the earlier of the two.
func TestBothTokensRequestsBothScopes(t *testing.T) {
	now := time.Now()
	cred := &scopeCredential{expires: map[string]time.Time{
		ExchangeScope: now.Add(time.Hour), GraphScope: now.Add(10 * time.Minute)}}
	token, err := bothTokens(context.Background(), cred)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cred.scopes, []string{ExchangeScope, GraphScope}) {
		t.Errorf("scopes = %v", cred.scopes)
	}
	if token.Value != "at-"+ExchangeScope || token.Graph != "at-"+GraphScope {
		t.Errorf("token = %+v", token)
	}
	if !token.ExpiresOn.Equal(now.Add(10 * time.Minute)) {
		t.Errorf("ExpiresOn = %v, want the Graph expiry (earlier)", token.ExpiresOn)
	}
}

// A failed Graph request fails Token: a worker without a Graph token would
// fail every cross-tenant call later with a less clear error.
func TestBothTokensGraphFailure(t *testing.T) {
	cred := &scopeCredential{fail: GraphScope, expires: map[string]time.Time{ExchangeScope: time.Now().Add(time.Hour)}}
	if _, err := bothTokens(context.Background(), cred); err == nil || !strings.Contains(err.Error(), "Microsoft Graph") {
		t.Fatalf("err = %v, want a Microsoft Graph error", err)
	}
}

// The Graph token reaches the bridge in the child environment, and a value
// inherited from the parent never does.
func TestBuildEnvPassesGraphToken(t *testing.T) {
	t.Setenv(EnvBridgeGraphToken, "inherited")
	c := newTestClient(time.Minute)
	c.config.TokenSource = &fakeTokenSource{tokens: []AccessToken{{Value: "at-1", Graph: "gt-1", ExpiresOn: time.Now().Add(time.Hour)}}}
	env, _, err := c.buildEnv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var graph []string
	for _, kv := range env {
		if strings.HasPrefix(kv, EnvBridgeGraphToken+"=") {
			graph = append(graph, kv)
		}
	}
	if !slices.Equal(graph, []string{EnvBridgeGraphToken + "=gt-1"}) {
		t.Errorf("graph token entries = %v", graph)
	}
}
