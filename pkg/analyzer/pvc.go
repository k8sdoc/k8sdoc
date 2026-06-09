package analyzer

import (
	"fmt"

	"github.com/user/k8sdoc/pkg/common"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PVCAnalyzer struct{}

func (PVCAnalyzer) Analyze(a common.Analyzer) ([]common.Result, error) {
	kind := "PersistentVolumeClaim"

	list, err := a.Client.GetClient().CoreV1().PersistentVolumeClaims(a.Namespace).List(
		a.Context,
		metav1.ListOptions{LabelSelector: a.LabelSelector},
	)
	if err != nil {
		return nil, err
	}

	var results []common.Result

	for _, pvc := range list.Items {
		if pvc.Status.Phase == v1.ClaimPending {
			results = append(results, common.Result{
				Kind: kind,
				Name: fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name),
				Error: []common.Failure{{
					Text: fmt.Sprintf("PVC %s/%s is in Pending state — no matching PersistentVolume or StorageClass",
						pvc.Namespace, pvc.Name),
				}},
			})
		}
	}
	return results, nil
}
