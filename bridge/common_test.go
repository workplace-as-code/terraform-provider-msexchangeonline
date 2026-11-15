package bridge

import (
	"os/exec"
	"strings"
	"testing"
)

// runCommon runs body in pwsh after loading scripts/common.ps1, with a
// Start-Sleep that returns at once.
func runCommon(t *testing.T, body string) string {
	t.Helper()
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh not installed")
	}
	script := ". ./scripts/common.ps1\nfunction Start-Sleep { param([int]$Seconds) }\n" + body
	out, err := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	return string(out)
}

// TestRetryNeverCreatesTwice guards Invoke-BridgeRetry: after a timeout or a
// lost connection, Exchange Online may have created the object, so a creation
// (-Create) is not sent again; a read is. A throttled or busy service rejected
// the request: a creation is sent again.
func TestRetryNeverCreatesTwice(t *testing.T) {
	out := runCommon(t, `
$script:n = 0
try { Invoke-BridgeRetry -Create { $script:n++; throw "The operation has timed out." } } catch { "create-timeout: $script:n $($_.Exception.Message)" }
$script:n = 0
try { Invoke-BridgeRetry -Create { $script:n++; throw "A task was canceled." } } catch { "create-canceled: $script:n $($_.Exception.Message)" }
$script:n = 0
$r = Invoke-BridgeRetry { $script:n++; if ($script:n -lt 2) { throw "The operation has timed out." }; "ok" }
"read-timeout: $script:n $r"
$script:n = 0
$r = Invoke-BridgeRetry { $script:n++; if ($script:n -lt 2) { throw "A task was canceled." }; "ok" }
"read-canceled: $script:n $r"
$script:n = 0
$r = Invoke-BridgeRetry -Create { $script:n++; if ($script:n -lt 2) { throw "The server is temporarily unavailable." }; "ok" }
"create-busy: $script:n $r"
`)
	for _, want := range []string{
		"create-timeout: 1 The operation has timed out.",
		"create-canceled: 1 A task was canceled.",
		"read-timeout: 2 ok",
		"read-canceled: 2 ok",
		"create-busy: 2 ok",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
