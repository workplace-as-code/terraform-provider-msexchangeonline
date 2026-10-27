package client

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExtractScriptsWritesEmbeddedFilesPrivately(t *testing.T) {
	dir, err := extractScripts(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"auth.ps1", "transport_rule.ps1"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s not extracted: %v", name, err)
		}
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(dir)
		if info.Mode().Perm() != 0o700 {
			t.Errorf("script dir permissions = %o, want 700", info.Mode().Perm())
		}
	}
}

func TestExtractScriptsRepairsTamperedFile(t *testing.T) {
	base := t.TempDir()
	dir, err := extractScripts(base)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "auth.ps1")
	if err := os.WriteFile(target, []byte("malicious"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := extractScripts(base); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(target)
	if string(data) == "malicious" {
		t.Fatal("tampered script was not replaced by the embedded copy")
	}
}

func TestSecretsAreNeverOnTheCommandLine(t *testing.T) {
	c := &Client{config: Config{TenantID: "t", ClientID: "c", CertificatePath: "/x.pfx", CertificatePassword: "s3cr3t-cert"}}
	_, args := c.buildArgs("/scripts/x.ps1", map[string]string{"Action": "Get"})
	if joined := strings.Join(args, " "); strings.Contains(joined, "s3cr3t") {
		t.Errorf("secret leaked into args: %s", joined)
	}
	env, _, err := c.buildEnv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(env, "\n"), "s3cr3t") {
		t.Error("secret missing from child environment")
	}
}

// fakeTokenSource hands out the given tokens in order, then keeps returning
// the last one. It counts the calls.
type fakeTokenSource struct {
	tokens []AccessToken
	err    error
	calls  int
}

func (f *fakeTokenSource) Token(context.Context) (AccessToken, error) {
	f.calls++
	if f.err != nil {
		return AccessToken{}, f.err
	}
	i := min(f.calls, len(f.tokens)) - 1
	return f.tokens[i], nil
}

func TestBridgeEnvDropsInheritedBridgeSecrets(t *testing.T) {
	t.Setenv(EnvBridgeAccessToken, "stale")
	t.Setenv("EXCHANGE_OIDC_TOKEN", "jwt")
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "gh")
	t.Setenv("IDENTITY_HEADER", "app-service-secret")
	t.Setenv("MSI_SECRET", "azure-ml-secret")
	c := &Client{config: Config{CertificatePath: "/x.pfx"}}
	env, expiresOn, err := c.buildEnv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, kv := range env {
		if kv == EnvBridgeAccessToken+"=stale" || strings.HasPrefix(kv, "EXCHANGE_OIDC_TOKEN=") || strings.HasPrefix(kv, "ACTIONS_ID_TOKEN_REQUEST_TOKEN=") ||
			strings.HasPrefix(kv, "IDENTITY_HEADER=") || strings.HasPrefix(kv, "MSI_SECRET=") {
			t.Errorf("the bridge environment still contains %s", kv)
		}
	}
	if !expiresOn.IsZero() {
		t.Errorf("no token source, yet an expiry: %v", expiresOn)
	}
}

func TestRunScriptPassesTheAccessTokenThroughEnvironment(t *testing.T) {
	fakePwsh(t, `echo "$`+EnvBridgeAccessToken+`"`)
	c := newTestClient(time.Minute)
	c.config.TokenSource = &fakeTokenSource{tokens: []AccessToken{{Value: "at-1", ExpiresOn: time.Now().Add(time.Hour)}}}
	out, err := c.RunScript(context.Background(), "x.ps1", nil)
	if err != nil || strings.TrimSpace(string(out)) != "at-1" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	_, args := c.buildArgs("/scripts/x.ps1", nil)
	if strings.Contains(strings.Join(args, " "), "at-1") {
		t.Fatal("access token leaked into the command line")
	}
}

func TestRunScriptPassesTheCertificateBase64ThroughEnvironment(t *testing.T) {
	fakePwsh(t, `echo "$`+EnvBridgeCertificateBase64+`"`)
	c := newTestClient(time.Minute)
	c.config.CertificateBase64 = "TUlJQ2VkLXBmeA=="
	out, err := c.RunScript(context.Background(), "x.ps1", nil)
	if err != nil || strings.TrimSpace(string(out)) != "TUlJQ2VkLXBmeA==" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	_, args := c.buildArgs("/scripts/x.ps1", nil)
	if strings.Contains(strings.Join(args, " "), "TUlJQ2VkLXBmeA==") {
		t.Fatal("certificate leaked into the command line")
	}
}

func TestBuildArgsIsDeterministic(t *testing.T) {
	c := &Client{}
	params := map[string]string{"B": "2", "A": "1", "C": "3"}
	_, first := c.buildArgs("x.ps1", params)
	for i := 0; i < 20; i++ {
		_, again := c.buildArgs("x.ps1", params)
		if strings.Join(again, " ") != strings.Join(first, " ") {
			t.Fatal("argument order changes between calls")
		}
	}
	if !strings.HasSuffix(strings.Join(first, " "), "-A 1 -B 2 -C 3") {
		t.Errorf("params not sorted: %v", first)
	}
}

func TestIsNotFound(t *testing.T) {
	if !IsNotFound(nil) || !IsNotFound([]byte(" \r\n")) {
		t.Error("empty output must mean not found")
	}
	if IsNotFound([]byte("WARNING: something")) || IsNotFound([]byte(`{"Name":"x"}`)) {
		t.Error("non-empty output must never mean not found")
	}
}

func TestEnvParsing(t *testing.T) {
	t.Setenv(EnvScriptTimeout, "")
	if d, _ := durationFromEnv(EnvScriptTimeout, time.Minute); d != time.Minute {
		t.Errorf("default not applied: %s", d)
	}
	t.Setenv(EnvScriptTimeout, "nope")
	if _, err := durationFromEnv(EnvScriptTimeout, time.Minute); err == nil {
		t.Error("invalid duration accepted")
	}
	t.Setenv(EnvMaxConcurrency, "0")
	if _, err := intFromEnv(EnvMaxConcurrency, 4); err == nil {
		t.Error("zero concurrency accepted")
	}
}

// fakePwsh puts a fake "pwsh" executable first on PATH. It runs the given
// shell body, which lets us test RunScript without PowerShell or Exchange.
func fakePwsh(t *testing.T, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake pwsh uses a POSIX shell script")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pwsh"), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func newTestClient(timeout time.Duration) *Client {
	return &Client{scriptDir: "/scripts", timeout: timeout, sem: make(chan struct{}, 1)}
}

func TestRunScriptReturnsStdout(t *testing.T) {
	fakePwsh(t, `echo '{"ok":true}'`)
	out, err := newTestClient(time.Minute).RunScript(context.Background(), "x.ps1", nil)
	if err != nil || strings.TrimSpace(string(out)) != `{"ok":true}` {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestRunScriptSurfacesStderrOnFailure(t *testing.T) {
	fakePwsh(t, `echo "throttled by Exchange" >&2; exit 1`)
	_, err := newTestClient(time.Minute).RunScript(context.Background(), "x.ps1", nil)
	if err == nil || !strings.Contains(err.Error(), "throttled by Exchange") {
		t.Fatalf("expected stderr in error, got %v", err)
	}
}

func TestRunScriptTimesOut(t *testing.T) {
	fakePwsh(t, `exec sleep 30`)
	start := time.Now()
	_, err := newTestClient(300*time.Millisecond).RunScript(context.Background(), "x.ps1", nil)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if time.Since(start) > 10*time.Second {
		t.Fatal("timeout did not stop the process")
	}
}

func TestRunScriptPassesSecretsThroughEnvironment(t *testing.T) {
	fakePwsh(t, `echo "$`+EnvBridgeCertificatePassword+`"`)
	c := newTestClient(time.Minute)
	c.config.CertificatePassword = "from-env"
	out, err := c.RunScript(context.Background(), "x.ps1", nil)
	if err != nil || strings.TrimSpace(string(out)) != "from-env" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

// flakyPwsh fakes a PowerShell that fails once with a transient Exchange
// Online error, then succeeds. It counts its runs in a file.
func flakyPwsh(t *testing.T) string {
	t.Helper()
	counter := filepath.Join(t.TempDir(), "runs")
	t.Setenv("FAKE_RUNS", counter)
	fakePwsh(t, `n=$(cat "$FAKE_RUNS" 2>/dev/null || echo 0); n=$((n+1)); echo $n > "$FAKE_RUNS"
if [ $n -lt 2 ]; then echo "A server side error has occurred. Please try again after some time." >&2; exit 1; fi
echo '{"ok":true}'`)
	previous := readRetryDelay
	readRetryDelay = 10 * time.Millisecond
	t.Cleanup(func() { readRetryDelay = previous })
	return counter
}

func runs(t *testing.T, counter string) string {
	t.Helper()
	data, err := os.ReadFile(counter)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(data))
}

func TestRunScriptRetriesATransientRead(t *testing.T) {
	counter := flakyPwsh(t)
	out, err := newTestClient(time.Minute).RunScript(context.Background(), "x.ps1", map[string]string{"Action": "Get"})
	if err != nil || !strings.Contains(string(out), "ok") {
		t.Fatalf("read was not retried: %v", err)
	}
	if got := runs(t, counter); got != "2" {
		t.Errorf("runs = %s, want 2", got)
	}
}

func TestRunScriptNeverRetriesAWrite(t *testing.T) {
	counter := flakyPwsh(t)
	if _, err := newTestClient(time.Minute).RunScript(context.Background(), "x.ps1", map[string]string{"Action": "Create"}); err == nil {
		t.Fatal("a write must fail on the first transient error")
	}
	if got := runs(t, counter); got != "1" {
		t.Errorf("runs = %s, want 1: a write must never be run twice", got)
	}
}
