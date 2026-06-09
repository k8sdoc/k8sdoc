package analyzer

import (
	"fmt"

	"github.com/user/k8sdoc/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// It detects Ingress objects whose backend services are missing
type IngressAnalyzer struct{}

func (IngressAnalyzer) Analyze(a common.Analyzer) ([]common.Result, error) {
	kind := "Ingress"

	list, err := a.Client.GetClient().NetworkingV1().Ingresses(a.Namespace).List(
		a.Context,
		metav1.ListOptions{LabelSelector: a.LabelSelector},
	)
	if err != nil {
		return nil, err
	}

	var results []common.Result

	for _, ing := range list.Items {
		var failures []common.Failure
		for _, rule := range ing.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}
			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service == nil {
					continue
				}
				svcName := path.Backend.Service.Name
				_, err := a.Client.GetClient().CoreV1().Services(ing.Namespace).Get(
					a.Context, svcName, metav1.GetOptions{},
				)
				if err != nil {
					failures = append(failures, common.Failure{
						Text: fmt.Sprintf("Ingress backend service %s/%s not found for path %s",
							ing.Namespace, svcName, path.Path),
					})
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, common.Result{
				Kind:  kind,
				Name:  fmt.Sprintf("%s/%s", ing.Namespace, ing.Name),
				Error: failures,
			})
		}
	}
	return results, nil
}
