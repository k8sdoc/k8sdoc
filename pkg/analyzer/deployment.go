package analyzer

import (
	"fmt"

	"github.com/user/k8sdoc/pkg/common"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// It detects deployments with unavailable replicas, stalled rollouts, or mismatched selectors
type DeploymentAnalyzer struct{}

func (DeploymentAnalyzer) Analyze(a common.Analyzer) ([]common.Result, error) {
	kind := "Deployment"

	list, err := a.Client.GetClient().AppsV1().Deployments(a.Namespace).List(
		a.Context,
		metav1.ListOptions{LabelSelector: a.LabelSelector},
	)
	if err != nil {
		return nil, err
	}

	var results []common.Result

	for _, dep := range list.Items {
		var failures []common.Failure

		desired := int32(1)
		if dep.Spec.Replicas != nil {
			desired = *dep.Spec.Replicas
		}
		available := dep.Status.AvailableReplicas
		ready := dep.Status.ReadyReplicas

		if available < desired {
			failures = append(failures, common.Failure{
				Text: fmt.Sprintf("Deployment has %d/%d available replicas", available, desired),
			})
		}
		if ready < desired {
			failures = append(failures, common.Failure{
				Text: fmt.Sprintf("Deployment has %d/%d ready replicas", ready, desired),
			})
		}

		for _, cond := range dep.Status.Conditions {
			if cond.Type == appsv1.DeploymentProgressing &&
				cond.Status == "False" &&
				cond.Reason == "ProgressDeadlineExceeded" {
				failures = append(failures, common.Failure{
					Text: fmt.Sprintf("Deployment rollout stalled: %s", cond.Message),
				})
			}
			if cond.Type == appsv1.DeploymentAvailable && cond.Status == "False" {
				failures = append(failures, common.Failure{
					Text: fmt.Sprintf("Deployment not available: %s", cond.Message),
				})
			}
		}

		if len(failures) > 0 {
			results = append(results, common.Result{
				Kind:  kind,
				Name:  fmt.Sprintf("%s/%s", dep.Namespace, dep.Name),
				Error: failures,
			})
		}
	}
	return results, nil
}
