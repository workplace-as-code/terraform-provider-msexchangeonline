package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const (
	// ExchangeScope is the scope of an access token for Exchange Online in the
	// public cloud (also used by GCC).
	ExchangeScope = "https://outlook.office365.com/.default"
	// GitHubAudience is the audience Entra ID expects in an OIDC token it
	// exchanges, in the public cloud.
	GitHubAudience = "api://AzureADTokenExchange"
)

// AccessToken is an Exchange Online access token and the time it expires.
type AccessToken struct {
	Value     string
	ExpiresOn time.Time
}

// TokenSource returns an Exchange Online access token. The client asks for one
// each time it starts a PowerShell process; the source caches the token and
// renews it before it expires.
//
// An interface is a set of methods: any type that has a Token method with this
// signature is a TokenSource. The tests use a fake one, OIDC uses
// oidcTokenSource, and the managed identity will add another.
type TokenSource interface {
	Token(ctx context.Context) (AccessToken, error)
}

// AssertionFunc returns the OIDC token that Entra ID exchanges for an access
// token. It is called each time a new access token is needed, so a source that
// renews its token (GitHub, a Kubernetes token file) is read again.
type AssertionFunc func(ctx context.Context) (string, error)

// StaticAssertion returns a token passed as a fixed value, without the blanks
// around it (HCL file() keeps the final newline). It cannot be renewed: once
// it expires, no new access token can be obtained.
func StaticAssertion(token string) AssertionFunc {
	token = strings.TrimSpace(token)
	// The returned function "captures" token: it keeps a reference to it after
	// StaticAssertion has returned (a closure).
	return func(context.Context) (string, error) { return token, nil }
}

// FileAssertion returns the token stored in a file, read again on every call.
func FileAssertion(path string) AssertionFunc {
	return func(context.Context) (string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("reading the OIDC token file %s: %w", path, err)
		}
		return strings.TrimSpace(string(data)), nil
	}
}

// githubHTTPClient bounds the request for the job's OIDC token.
var githubHTTPClient = &http.Client{Timeout: 30 * time.Second}

// GitHubAssertion requests the job's OIDC token from GitHub Actions. requestURL
// and requestToken are the ACTIONS_ID_TOKEN_REQUEST_URL and
// ACTIONS_ID_TOKEN_REQUEST_TOKEN variables GitHub sets in a job that has
// "permissions: id-token: write".
func GitHubAssertion(requestURL, requestToken string) AssertionFunc {
	return func(ctx context.Context) (string, error) {
		// The request URL already carries a query string (api-version).
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL+"&audience="+url.QueryEscape(GitHubAudience), nil)
		if err != nil {
			return "", fmt.Errorf("requesting the OIDC token from GitHub Actions: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+requestToken)
		req.Header.Set("Accept", "application/json; api-version=2.0")

		resp, err := githubHTTPClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("requesting the OIDC token from GitHub Actions: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		// The body is never quoted in an error: it could echo a credential.
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf(`requesting the OIDC token from GitHub Actions failed (HTTP %d): check that the job has "permissions: id-token: write"`, resp.StatusCode)
		}
		var body struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Value == "" {
			return "", errors.New("GitHub Actions returned no OIDC token")
		}
		return body.Value, nil
	}
}

// oidcCredentialOptions configures the azidentity credential. nil keeps the
// library's defaults; tests replace it to answer without the network.
var oidcCredentialOptions *azidentity.ClientAssertionCredentialOptions

// oidcTokenSource exchanges an OIDC token for an Exchange Online access token.
type oidcTokenSource struct {
	credential *azidentity.ClientAssertionCredential
	hint       string
}

// NewOIDCTokenSource returns a TokenSource for the app registration clientID
// in the tenant organization (its .onmicrosoft.com domain). hint, when not
// empty, is added to the error when no token can be obtained.
//
// azidentity, Microsoft's library, does the exchange: it caches the access
// token and calls assertion again when it needs a new one.
func NewOIDCTokenSource(organization, clientID string, assertion AssertionFunc, hint string) (TokenSource, error) {
	var options azidentity.ClientAssertionCredentialOptions
	if oidcCredentialOptions != nil {
		options = *oidcCredentialOptions
	}
	// An explicit cloud makes azidentity ignore AZURE_AUTHORITY_HOST: a
	// variable left in a CI runner cannot send the request to another cloud.
	options.Cloud = cloud.AzurePublic
	credential, err := azidentity.NewClientAssertionCredential(organization, clientID, assertion, &options)
	if err != nil {
		return nil, err
	}
	return &oidcTokenSource{credential: credential, hint: hint}, nil
}

func (s *oidcTokenSource) Token(ctx context.Context) (AccessToken, error) {
	token, err := s.credential.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{ExchangeScope}})
	if err != nil {
		msg := "obtaining an Exchange Online access token with OIDC failed: " + err.Error()
		if s.hint != "" {
			msg += ". " + s.hint
		}
		return AccessToken{}, errors.New(msg)
	}
	return AccessToken{Value: token.Token, ExpiresOn: token.ExpiresOn}, nil
}
