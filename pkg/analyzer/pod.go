package analyzer

import (
	"fmt"

	"github.com/user/k8sdoc/pkg/common"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// It detects pods in failing states: CrashLoopBackOff, OOMKilled, ImagePullBackOff, Pending/Unschedulable, and Evicted.
type PodAnalyzer struct{}

func (PodAnalyzer) Analyze(a common.Analyzer) ([]common.Result, error) {
	kind := "Pod"

	list, err := a.Client.GetClient().CoreV1().Pods(a.Namespace).List(
		a.Context,
		metav1.ListOptions{LabelSelector: a.LabelSelector},
	)
	if err != nil {
		return nil, err
	}

	var preAnalysis = map[string]common.PreAnalysis{}

	for _, pod := range list.Items {
		var failures []common.Failure

		if pod.Status.Reason == "Evicted" {
			msg := fmt.Sprintf("Pod %s/%s has been evicted", pod.Namespace, pod.Name)
			if pod.Status.Message != "" {
				msg = pod.Status.Message
			}
			failures = append(failures, common.Failure{Text: msg})
		}

		if pod.Status.Phase == v1.PodPending {
			for _, cond := range pod.Status.Conditions {
				if cond.Type == v1.PodScheduled && cond.Reason == "Unschedulable" && cond.Message != "" {
					failures = append(failures, common.Failure{Text: cond.Message})
				}
			}
		}

		failures = append(failures, containerStatusFailures(pod.Status.InitContainerStatuses)...)
		failures = append(failures, containerStatusFailures(pod.Status.ContainerStatuses)...)

		if len(failures) > 0 {
			key := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
			preAnalysis[key] = common.PreAnalysis{Pod: pod, FailureDetails: failures}
		}
	}

	var results []common.Result
	for key, pre := range preAnalysis {
		results = append(results, common.Result{
			Kind:  kind,
			Name:  key,
			Error: pre.FailureDetails,
		})
	}
	return results, nil
}

func containerStatusFailures(statuses []v1.ContainerStatus) []common.Failure {
	var failures []common.Failure
	for _, cs := range statuses {
		if cs.State.Waiting != nil {
			switch cs.State.Waiting.Reason {
			case "CrashLoopBackOff":
				failures = append(failures, common.Failure{
					Text: fmt.Sprintf("Container %s is in CrashLoopBackOff: %s",
						cs.Name, cs.State.Waiting.Message),
				})
			case "ImagePullBackOff", "ErrImagePull":
				failures = append(failures, common.Failure{
					Text: fmt.Sprintf("Container %s cannot pull image: %s",
						cs.Name, cs.State.Waiting.Message),
				})
			case "OOMKilled":
				failures = append(failures, common.Failure{
					Text: fmt.Sprintf("Container %s was OOMKilled — increase memory limits", cs.Name),
				})
			}
		}
		if cs.State.Terminated != nil && cs.State.Terminated.Reason == "OOMKilled" {
			failures = append(failures, common.Failure{
				Text: fmt.Sprintf("Container %s was OOMKilled (exit %d) — increase memory limits",
					cs.Name, cs.State.Terminated.ExitCode),
			})
		}
		if cs.RestartCount > 5 {
			failures = append(failures, common.Failure{
				Text: fmt.Sprintf("Container %s has restarted %d times", cs.Name, cs.RestartCount),
			})
		}
	}
	return failures
}
