package analyzer

import (
	"fmt"

	"github.com/user/k8sdoc/pkg/common"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// It detects nodes in NotReady state, memory/disk/PID pressure, and nodes marked unschedulable.
type NodeAnalyzer struct{}

func (NodeAnalyzer) Analyze(a common.Analyzer) ([]common.Result, error) {
	kind := "Node"

	list, err := a.Client.GetClient().CoreV1().Nodes().List(
		a.Context,
		metav1.ListOptions{},
	)
	if err != nil {
		return nil, err
	}

	var results []common.Result

	for _, node := range list.Items {
		var failures []common.Failure

		for _, cond := range node.Status.Conditions {
			switch cond.Type {
			case v1.NodeReady:
				if cond.Status == v1.ConditionFalse || cond.Status == v1.ConditionUnknown {
					failures = append(failures, common.Failure{
						Text: fmt.Sprintf("Node is %s: %s — %s",
							cond.Status, cond.Reason, cond.Message),
					})
				}
			case v1.NodeMemoryPressure:
				if cond.Status == v1.ConditionTrue {
					failures = append(failures, common.Failure{
						Text: fmt.Sprintf("Node has MemoryPressure: %s", cond.Message),
					})
				}
			case v1.NodeDiskPressure:
				if cond.Status == v1.ConditionTrue {
					failures = append(failures, common.Failure{
						Text: fmt.Sprintf("Node has DiskPressure: %s", cond.Message),
					})
				}
			case v1.NodePIDPressure:
				if cond.Status == v1.ConditionTrue {
					failures = append(failures, common.Failure{
						Text: fmt.Sprintf("Node has PIDPressure: %s", cond.Message),
					})
				}
			case v1.NodeNetworkUnavailable:
				if cond.Status == v1.ConditionTrue {
					failures = append(failures, common.Failure{
						Text: fmt.Sprintf("Node network unavailable: %s", cond.Message),
					})
				}
			}
		}

		if node.Spec.Unschedulable {
			failures = append(failures, common.Failure{
				Text: fmt.Sprintf("Node %s is cordoned (unschedulable)", node.Name),
			})
		}

		if len(failures) > 0 {
			results = append(results, common.Result{
				Kind:  kind,
				Name:  node.Name,
				Error: failures,
			})
		}
	}
	return results, nil
}
