package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
