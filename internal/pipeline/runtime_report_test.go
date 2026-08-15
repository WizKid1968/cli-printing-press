package pipeline

import "testing"


// A cookie-auth CLI that builds cleanly must not be discarded just because no
// human ran `auth login --chrome`. Measured in production: an autotempest press
// scored 96% (27/28, 1 critical) and was refunded on this gate alone.
func TestBrowserSessionMissingWarnsWhenOtherwiseHealthy(t *testing.T) {
	report := &VerifyReport{
		Results:               []CommandResult{{Score: 3}, {Score: 3}, {Score: 3}},
		BrowserSessionRequired: true,
		BrowserSessionProof:    "missing",
	}
	finalizeVerifyReport(report, 60, false)
	if report.Verdict != "WARN" {
		t.Fatalf("healthy cookie-auth CLI got %q, want WARN (it is shippable with a login caveat)", report.Verdict)
	}
}

// But a genuinely broken CLI still fails — the missing proof must not rescue it.
func TestBrowserSessionMissingStillFailsWhenBroken(t *testing.T) {
	report := &VerifyReport{
		Results:               []CommandResult{{Score: 0}, {Score: 0}, {Score: 0}},
		BrowserSessionRequired: true,
		BrowserSessionProof:    "missing",
	}
	finalizeVerifyReport(report, 60, false)
	if report.Verdict != "FAIL" {
		t.Fatalf("broken CLI got %q, want FAIL", report.Verdict)
	}
}
