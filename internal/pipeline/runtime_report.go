package pipeline

func finalizeVerifyReport(report *VerifyReport, threshold int, requireDataPipeline bool) {
	for _, result := range report.Results {
		report.Total++
		if result.Score >= 2 {
			report.Passed++
			continue
		}
		report.Failed++
		if result.Score == 0 {
			report.Critical++
		}
	}
	// Path-param probes catch the failure mode where a nested leaf command
	// exits with a "<positional> is required" usage error even though the
	// caller supplied the positionals. The existing per-command matrix
	// only probes top-level commands, so this gap let a generator codegen
	// bug (mis-indexed args[] in path-param emit) ship silently with
	// verify reporting 100% pass. Each failing probe is a critical
	// failure: the command is unusable as shipped.
	for _, probe := range report.PathParamProbes {
		report.Total++
		if probe.Passed {
			report.Passed++
			continue
		}
		report.Failed++
		report.Critical++
	}
	if report.Total > 0 {
		report.PassRate = float64(report.Passed) / float64(report.Total) * 100
	}

	commandsGate := report.PassRate >= float64(threshold) && report.Critical == 0
	passGate := commandsGate
	if requireDataPipeline {
		passGate = passGate && report.DataPipeline
	}
	switch {
	// A data-pipeline failure alone no longer vetoes an otherwise healthy CLI.
	// `sync` is auxiliary scaffolding — an offline-cache convenience — not the
	// customer's API surface, and the deliverable is the command set. As an
	// absolute gate this discarded presses scoring 35/35 with zero critical
	// failures because one bulk-download probe ran out of budget, which punished
	// the richest APIs hardest and made verdicts irreproducible run to run.
	//
	// With every command passing and nothing critical, report WARN so the caller
	// ships with the caveat recorded. If the commands themselves are weak, the
	// gates below still fail the press on that evidence.
	case requireDataPipeline && !report.DataPipeline && commandsGate:
		report.Verdict = "WARN"
	case requireDataPipeline && !report.DataPipeline:
		report.Verdict = "FAIL"
	case passGate:
		report.Verdict = "PASS"
	case report.PassRate >= 60 && report.Critical <= 3:
		report.Verdict = "WARN"
	default:
		report.Verdict = "FAIL"
	}
	// A missing browser-session proof is a SETUP state, not a broken CLI, and as
	// an absolute veto it discarded healthy work. Measured: an autotempest press
	// scored 96% (27/28, 1 critical) and was refunded outright because no one had
	// run `auth login --chrome` — which an automated pipeline cannot do, since
	// the proof requires a human logging into a real browser. Every cookie-auth
	// site is unshippable under that rule no matter how well it builds.
	//
	// Same reasoning as the data-pipeline gate above: when the command surface is
	// otherwise healthy, report WARN so the caller ships with the caveat recorded
	// and tells the user to authenticate. If the commands themselves are weak,
	// the gates above have already failed it on that evidence.
	if report.BrowserSessionRequired && report.BrowserSessionProof != "valid" {
		if report.Verdict == "PASS" || report.Verdict == "WARN" {
			report.Verdict = "WARN"
		} else {
			report.Verdict = "FAIL"
		}
	}
}
