package analyzer

import (
	"fmt"

	"github.com/user/k8sdoc/pkg/common"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// It detects services with no ready endpoints (selector mismatch, pod crash, wrong targetPort) and warns about NodePort/LoadBalancer issues.
type ServiceAnalyzer struct{}

func (ServiceAnalyzer) Analyze(a common.Analyzer) ([]common.Result, error) {
	kind := "Service"

	epList, err := a.Client.GetClient().CoreV1().Endpoints(a.Namespace).List(
		a.Context,
		metav1.ListOptions{LabelSelector: a.LabelSelector},
	)
	if err != nil {
		return nil, err
	}

	var results []common.Result

	for _, ep := range epList.Items {
		if ep.Name == "kubernetes" {
			continue
		}

		readyAddrs := 0
		for _, subset := range ep.Subsets {
			readyAddrs += len(subset.Addresses)
		}

		if readyAddrs == 0 {
			svc, err := a.Client.GetClient().CoreV1().Services(ep.Namespace).Get(
				a.Context, ep.Name, metav1.GetOptions{},
			)
			if err != nil {
				continue
			}
			if len(svc.Spec.Selector) == 0 || svc.Spec.Type == v1.ServiceTypeExternalName {
				continue
			}

			var failures []common.Failure
			for k, v := range svc.Spec.Selector {
				failures = append(failures, common.Failure{
					Text: fmt.Sprintf(
						"Service %s/%s has no ready endpoints; selector %s=%s matched no running pods",
						svc.Namespace, svc.Name, k, v,
					),
				})
			}

			results = append(results, common.Result{
				Kind:  kind,
				Name:  fmt.Sprintf("%s/%s", ep.Namespace, ep.Name),
				Error: failures,
			})
		}
	}
	return results, nil
}
