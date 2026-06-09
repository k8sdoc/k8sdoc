package analyzer

import (
	"fmt"
	"strings"

	"github.com/user/k8sdoc/pkg/common"
)

// It looks at ProbeResults already collected by the KdoctorCollector and converts failures into structured Results that the AI layer can explain
type NetReachAnalyzer struct{}

func (NetReachAnalyzer) Analyze(a common.Analyzer) ([]common.Result, error) {
	kind := "NetReach"
	var results []common.Result

	for _, pr := range a.ProbeResults {
		if pr.TaskKind != "NetReach" {
			continue
		}
		if pr.Result != "fail" && pr.SuccessRate >= 0.99 {
			continue
		}

		var failures []common.Failure

		mainMsg := fmt.Sprintf(
			"NetReach task %q round %d failed: success_rate=%.1f%% latency_p95=%dms",
			pr.TaskName, pr.RoundNumber,
			pr.SuccessRate*100, pr.LatencyP95Ms,
		)
		failures = append(failures, common.Failure{Text: mainMsg})

		// List failed node pairs
		if len(pr.FailedNodes) > 0 {
			failures = append(failures, common.Failure{
				Text: fmt.Sprintf("Failed node/pod pairs: %s", strings.Join(pr.FailedNodes, ", ")),
			})
		}

		if pr.FailedReason != "" {
			failures = append(failures, common.Failure{
				Text: fmt.Sprintf("kdoctor failure reason: %s", pr.FailedReason),
			})
		}

		if pr.LatencyP95Ms > 200 {
			failures = append(failures, common.Failure{
				Text: fmt.Sprintf("High network latency detected: p95=%dms (threshold: 200ms)", pr.LatencyP95Ms),
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
