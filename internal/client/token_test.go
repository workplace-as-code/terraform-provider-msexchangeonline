package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

func TestStaticAssertion(t *testing.T) {
	got, err := StaticAssertion("jwt-static")(context.Background())
	if err != nil || got != "jwt-static" {
		t.Fatalf("got %q, %v", got, err)
	}
}

// HCL file() and pasted CI secrets keep the trailing newline: Entra ID would
// reject the token with an error that does not point at the cause.
func TestStaticAssertionTrims(t *testing.T) {
	got, err := StaticAssertion("  jwt-static\n")(context.Background())
	if err != nil || got != "jwt-static" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestFileAssertionTrimsAndRereads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.jwt")
	assertion := FileAssertion(path)

	if err := os.WriteFile(path, []byte("jwt-1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := assertion(context.Background()); err != nil || got != "jwt-1" {
		t.Fatalf("first read: got %q, %v", got, err)
	}

	// Kubernetes rewrites the projected token file: every exchange reads it again.
	if err := os.WriteFile(path, []byte("jwt-2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := assertion(context.Background()); err != nil || got != "jwt-2" {
		t.Fatalf("second read: got %q, %v", got, err)
	}

	missing := filepath.Join(t.TempDir(), "missing.jwt")
	if _, err := FileAssertion(missing)(context.Background()); err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("missing file: err = %v, want the path in the message", err)
	}
}

func TestGitHubAssertionRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("audience"); got != GitHubAudience {
			t.Errorf("audience = %q, want %q", got, GitHubAudience)
		}
		if got := r.URL.Query().Get("api-version"); got != "2.0" {
			t.Errorf("the query string of the request URL was lost: api-version = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer req-token" {
			t.Errorf("Authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"value":"gh-jwt"}`))
	}))
	defer server.Close()

	got, err := GitHubAssertion(server.URL+"/token?api-version=2.0", "req-token")(context.Background())
	if err != nil || got != "gh-jwt" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestGitHubAssertionErrorNamesPermission(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"bad token req-token"}`))
	}))
	defer server.Close()

	_, err := GitHubAssertion(server.URL+"/token?api-version=2.0", "req-token")(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "id-token: write") || !strings.Contains(msg, "403") {
		t.Errorf("error should name the permission and the status: %s", msg)
	}
	if strings.Contains(msg, "req-token") {
		t.Errorf("error leaks the request token: %s", msg)
	}
}

func TestNewOIDCTokenSourceAcceptsDomain(t *testing.T) {
	if _, err := NewOIDCTokenSource("contoso.onmicrosoft.com", "00000000-0000-0000-0000-000000000001", StaticAssertion("x"), ""); err != nil {
		t.Fatal(err)
	}
}

// offlineTransport fails every request at once, so a test of a token source
// never waits for Entra ID.
type offlineTransport struct{}

func (offlineTransport) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("offline")
}

func TestOIDCTokenSourceErrorCarriesHint(t *testing.T) {
	previous := oidcCredentialOptions
	oidcCredentialOptions = &azidentity.ClientAssertionCredentialOptions{ClientOptions: azcore.ClientOptions{
		Transport: offlineTransport{},
		Retry:     policy.RetryOptions{MaxRetries: -1},
	}}
	t.Cleanup(func() { oidcCredentialOptions = previous })

	failing := func(context.Context) (string, error) { return "", errors.New("no assertion") }
	source, err := NewOIDCTokenSource("contoso.onmicrosoft.com", "00000000-0000-0000-0000-000000000001", failing, "use the file")
	if err != nil {
		t.Fatal(err)
	}
	_, err = source.Token(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if msg := err.Error(); !strings.Contains(msg, "with OIDC failed") || !strings.Contains(msg, "use the file") {
		t.Errorf("error = %s", msg)
	}
}

// recordingTransport keeps the URL of every request, then fails it.
type recordingTransport struct{ urls []string }

func (r *recordingTransport) Do(req *http.Request) (*http.Response, error) {
	r.urls = append(r.urls, req.URL.String())
	return nil, errors.New("offline")
}

// A stray AZURE_AUTHORITY_HOST in a CI runner must not send the token request
// to another cloud: the provider only supports the public cloud with OIDC.
func TestOIDCTokenSourceIgnoresAuthorityHostVariable(t *testing.T) {
	t.Setenv("AZURE_AUTHORITY_HOST", "https://login.example.invalid/")
	transport := &recordingTransport{}
	previous := oidcCredentialOptions
	oidcCredentialOptions = &azidentity.ClientAssertionCredentialOptions{ClientOptions: azcore.ClientOptions{
		Transport: transport,
		Retry:     policy.RetryOptions{MaxRetries: -1},
	}}
	t.Cleanup(func() { oidcCredentialOptions = previous })

	source, err := NewOIDCTokenSource("contoso.onmicrosoft.com", "00000000-0000-0000-0000-000000000001", StaticAssertion("x"), "")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = source.Token(context.Background())
	if len(transport.urls) == 0 {
		t.Fatal("no request was sent")
	}
	// MSAL first asks login.microsoftonline.com about an unknown authority,
	// with that authority in the query string: check the whole URL.
	for _, u := range transport.urls {
		if !strings.HasPrefix(u, "https://login.microsoftonline.com/") || strings.Contains(u, "example.invalid") {
			t.Errorf("request for another authority: %s", u)
		}
	}
}

// clearManagedIdentityEnv empties the variables that select another managed
// identity API (App Service, Azure Arc, Cloud Shell, Service Fabric), so
// azidentity uses IMDS, the endpoint of an Azure VM, and the fake below.
func clearManagedIdentityEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"IDENTITY_ENDPOINT", "IDENTITY_HEADER", "IMDS_ENDPOINT", "MSI_ENDPOINT", "MSI_SECRET", "IDENTITY_SERVER_THUMBPRINT"} {
		t.Setenv(k, "")
	}
}

// imdsTransport answers every request like IMDS and keeps its URL.
//
// MSAL caches managed identity tokens for the whole process, across
// credentials. The token it returns expires in 2 minutes: MSAL never serves
// from its cache a token that expires within 5 minutes, so every test sends
// its own request whatever ran before.
type imdsTransport struct {
	requests []*url.URL
}

// imdsTokenLifetime is the lifetime of the tokens imdsTransport returns.
const imdsTokenLifetime = 2 * time.Minute

func (f *imdsTransport) Do(req *http.Request) (*http.Response, error) {
	f.requests = append(f.requests, req.URL)
	body := fmt.Sprintf(`{"access_token":"at-mi","expires_on":"%d","expires_in":"120","resource":"https://outlook.office365.com","token_type":"Bearer"}`, time.Now().Add(imdsTokenLifetime).Unix())
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

// useManagedIdentityTransport makes the managed identity credential send its
// requests to transport, without retries.
func useManagedIdentityTransport(t *testing.T, transport policy.Transporter) {
	t.Helper()
	previous := managedIdentityCredentialOptions
	managedIdentityCredentialOptions = &azidentity.ManagedIdentityCredentialOptions{ClientOptions: azcore.ClientOptions{
		Transport: transport,
		Retry:     policy.RetryOptions{MaxRetries: -1},
	}}
	t.Cleanup(func() { managedIdentityCredentialOptions = previous })
}

func TestManagedIdentityTokenSourceSystemAssigned(t *testing.T) {
	clearManagedIdentityEnv(t)
	transport := &imdsTransport{}
	useManagedIdentityTransport(t, transport)

	source, err := NewManagedIdentityTokenSource("")
	if err != nil {
		t.Fatal(err)
	}
	token, err := source.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if token.Value != "at-mi" {
		t.Errorf("token = %q", token.Value)
	}
	if d := time.Until(token.ExpiresOn); d < imdsTokenLifetime-time.Minute || d > imdsTokenLifetime {
		t.Errorf("token expires in %v, want the %v returned by IMDS", d, imdsTokenLifetime)
	}
	if len(transport.requests) != 1 {
		t.Fatalf("%d requests, want 1", len(transport.requests))
	}
	u := transport.requests[0]
	if u.Host != "169.254.169.254" || u.Query().Get("resource") != "https://outlook.office365.com" || u.Query().Has("client_id") {
		t.Errorf("request = %s, want IMDS for Exchange Online without client_id", u)
	}
}

func TestManagedIdentityTokenSourceUserAssigned(t *testing.T) {
	clearManagedIdentityEnv(t)
	transport := &imdsTransport{}
	useManagedIdentityTransport(t, transport)

	source, err := NewManagedIdentityTokenSource("11111111-2222-3333-4444-555555555555")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.Token(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := transport.requests[0].Query().Get("client_id"); got != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("client_id = %q", got)
	}
}

// AZURE_CLIENT_ID left in a runner (for another tool) must not turn the
// system-assigned identity into a user-assigned one.
func TestManagedIdentityIgnoresAzureClientID(t *testing.T) {
	clearManagedIdentityEnv(t)
	t.Setenv("AZURE_CLIENT_ID", "99999999-9999-9999-9999-999999999999")
	transport := &imdsTransport{}
	useManagedIdentityTransport(t, transport)

	source, err := NewManagedIdentityTokenSource("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.Token(context.Background()); err != nil {
		t.Fatal(err)
	}
	if u := transport.requests[0]; u.Query().Has("client_id") {
		t.Errorf("request = %s, want no client_id", u)
	}
}

func TestManagedIdentityErrorCarriesHint(t *testing.T) {
	clearManagedIdentityEnv(t)
	useManagedIdentityTransport(t, offlineTransport{})

	for _, clientID := range []string{"", "11111111-2222-3333-4444-555555555555"} {
		source, err := NewManagedIdentityTokenSource(clientID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = source.Token(context.Background())
		if err == nil {
			t.Fatal("expected an error")
		}
		msg := err.Error()
		for _, want := range []string{"with the managed identity failed", "Azure Arc", "Hybrid agent extension applications"} {
			if !strings.Contains(msg, want) {
				t.Errorf("client_id %q: error lacks %q: %s", clientID, want, msg)
			}
		}
		if got := strings.Contains(msg, "attached"); got != (clientID != "") {
			t.Errorf("client_id %q: user-assigned hint present = %v: %s", clientID, got, msg)
		}
	}
}

// Outside Azure nothing answers on the IMDS address: azidentity's defaults
// (6 retries, 1-minute tries) would keep the run waiting for minutes.
func TestManagedIdentityRetryIsBounded(t *testing.T) {
	o := managedIdentityOptions("")
	if o.Retry.MaxRetries != 3 || o.Retry.TryTimeout != 5*time.Second {
		t.Errorf("retry = %+v, want 3 retries of 5s", o.Retry)
	}
	if o.ID != nil {
		t.Errorf("ID = %v, want nil (system-assigned)", o.ID)
	}
	if o := managedIdentityOptions("x"); o.ID != azidentity.ClientID("x") {
		t.Errorf("ID = %v, want ClientID(x)", o.ID)
	}
}
