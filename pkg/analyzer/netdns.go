package analyzer

import (
	"fmt"
	"strings"

	"github.com/user/k8sdoc/pkg/common"
)

// It wraps kdoctor Netdns probe results, correlating DNS query failures with CoreDNS pod health
type NetDNSAnalyzer struct{}

func (NetDNSAnalyzer) Analyze(a common.Analyzer) ([]common.Result, error) {
	kind := "Netdns"
	var results []common.Result

	for _, pr := range a.ProbeResults {
		if pr.TaskKind != "Netdns" {
			continue
		}
		if pr.Result != "fail" && pr.SuccessRate >= 0.99 {
			continue
		}

		var failures []common.Failure

		failures = append(failures, common.Failure{
			Text: fmt.Sprintf(
				"Netdns task %q round %d: DNS success_rate=%.1f%%",
				pr.TaskName, pr.RoundNumber, pr.SuccessRate*100,
			),
		})

		if len(pr.FailedNodes) > 0 {
			failures = append(failures, common.Failure{
				Text: fmt.Sprintf("DNS failures from agent pods: %s", strings.Join(pr.FailedNodes, ", ")),
			})
		}

		if pr.FailedReason != "" {
			reason := pr.FailedReason
			switch {
			case strings.Contains(reason, "NXDOMAIN"):
				failures = append(failures, common.Failure{
					Text: "NXDOMAIN errors - possible CoreDNS misconfiguration or missing Service entry",
				})
			case strings.Contains(reason, "SERVFAIL"):
				failures = append(failures, common.Failure{
					Text: "SERVFAIL responses - CoreDNS pod may be crashing or upstream DNS unreachable",
				})
			case strings.Contains(reason, "timeout"):
				failures = append(failures, common.Failure{
					Text: "DNS query timeouts - check CoreDNS pod resource limits and UDP connectivity",
				})
			default:
				failures = append(failures, common.Failure{Text: reason})
			}
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
