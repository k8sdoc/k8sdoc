package analyzer

import (
	"fmt"
	"strings"

	"github.com/user/k8sdoc/pkg/common"
)

// AppHttpHealthyAnalyzer wraps kdoctor AppHttpHealthy probe results flagging HTTP 5xx rates, TLS errors, high latency, and DNS failures.
type AppHttpHealthyAnalyzer struct{}

func (AppHttpHealthyAnalyzer) Analyze(a common.Analyzer) ([]common.Result, error) {
	kind := "AppHttpHealthy"
	var results []common.Result

	for _, pr := range a.ProbeResults {
		if pr.TaskKind != "AppHttpHealthy" {
			continue
		}
		if pr.Result != "fail" && pr.SuccessRate >= 0.99 {
			continue
		}

		var failures []common.Failure

		failures = append(failures, common.Failure{
			Text: fmt.Sprintf(
				"AppHttpHealthy task %q round %d: success_rate=%.1f%% latency_p95=%dms",
				pr.TaskName, pr.RoundNumber,
				pr.SuccessRate*100, pr.LatencyP95Ms,
			),
		})

		if len(pr.FailedNodes) > 0 {
			failures = append(failures, common.Failure{
				Text: fmt.Sprintf("Failing agent pods: %s", strings.Join(pr.FailedNodes, ", ")),
			})
		}

		if pr.FailedReason != "" {
			reason := pr.FailedReason
			switch {
			case strings.Contains(reason, "tls") || strings.Contains(reason, "certificate"):
				failures = append(failures, common.Failure{
					Text: fmt.Sprintf("TLS/certificate error detected: %s", reason),
				})
			case strings.Contains(reason, "dns") || strings.Contains(reason, "no such host"):
				failures = append(failures, common.Failure{
					Text: fmt.Sprintf("DNS resolution failure: %s", reason),
				})
			case strings.Contains(reason, "timeout"):
				failures = append(failures, common.Failure{
					Text: fmt.Sprintf("Request timeout: %s", reason),
				})
			case strings.Contains(reason, "connection refused"):
				failures = append(failures, common.Failure{
					Text: fmt.Sprintf("Connection refused — target pod may be down: %s", reason),
				})
			default:
				failures = append(failures, common.Failure{Text: reason})
			}
		}

		if pr.LatencyP95Ms > 500 {
			failures = append(failures, common.Failure{
				Text: fmt.Sprintf("High HTTP latency: p95=%dms (threshold: 500ms)", pr.LatencyP95Ms),
			})
		}

		results = append(results, common.Result{
			Kind:     kind,
			Name:     pr.TaskName,
			Error:    failures,
			ProbeRef: pr.TaskName,
		})
	}
	return results, nil
}
