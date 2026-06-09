package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/user/k8sdoc/pkg/common"
	"github.com/user/k8sdoc/pkg/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	gvrNetReach = schema.GroupVersionResource{
		Group:    "kdoctor.io",
		Version:  "v1beta1",
		Resource: "netreaches",
	}
	gvrAppHttp = schema.GroupVersionResource{
		Group:    "kdoctor.io",
		Version:  "v1beta1",
		Resource: "apphttphealthies",
	}
	gvrNetDNS = schema.GroupVersionResource{
		Group:    "kdoctor.io",
		Version:  "v1beta1",
		Resource: "netdnses",
	}
	gvrKdoctorReport = schema.GroupVersionResource{
		Group:    "system.kdoctor.io",
		Version:  "v1beta1",
		Resource: "kdoctorreports",
	}
)


type KdoctorCollector struct {
	client    *kubernetes.Client
	namespace string
}

func NewKdoctorCollector(client *kubernetes.Client, namespace string) *KdoctorCollector {
	return &KdoctorCollector{client: client, namespace: namespace}
}

func (kc *KdoctorCollector) Collect(ctx context.Context) ([]common.ProbeResult, error) {
	var results []common.ProbeResult

	nr, err := kc.collectReports(ctx, gvrNetReach, "NetReach")
	if err != nil {
		return nil, fmt.Errorf("collecting NetReach: %w", err)
	}
	results = append(results, nr...)

	ah, err := kc.collectReports(ctx, gvrAppHttp, "AppHttpHealthy")
	if err != nil {
		return nil, fmt.Errorf("collecting AppHttpHealthy: %w", err)
	}
	results = append(results, ah...)

	nd, err := kc.collectReports(ctx, gvrNetDNS, "Netdns")
	if err != nil {
		return nil, fmt.Errorf("collecting Netdns: %w", err)
	}
	results = append(results, nd...)

	return results, nil
}


func (kc *KdoctorCollector) collectReports(ctx context.Context, gvr schema.GroupVersionResource, taskKind string) ([]common.ProbeResult, error) {
	dynClient := kc.client.GetDynamicClient()

	taskList, err := dynClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}

	var results []common.ProbeResult

	for _, item := range taskList.Items {
		taskName := item.GetName()

		reportRaw, err := dynClient.Resource(gvrKdoctorReport).Get(ctx, taskName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		raw, err := json.Marshal(reportRaw.Object)
		if err != nil {
			continue
		}
		var report KdoctorReport
		if err := json.Unmarshal(raw, &report); err != nil {
			continue
		}

		if report.Report.LatestRoundReport == nil {
			continue
		}

		pr := kc.aggregateRound(taskName, taskKind, *report.Report.LatestRoundReport)
		if pr != nil {
			results = append(results, *pr)
		}
	}
	return results, nil
}

func (kc *KdoctorCollector) aggregateRound(taskName, taskKind string, reports []RoundReport) *common.ProbeResult {
	if len(reports) == 0 {
		return nil
	}

	var (
		totalSucceed int64
		totalFailed  int64
		failedNodes  []string
		failedReason string
		latencyP95   int64
		roundNum     int64
		roundDur     string
		ts           time.Time
	)

	for _, r := range reports {
		roundNum = r.RoundNumber
		roundDur = r.RoundDuration
		ts = r.StartTimeStamp.Time

		if r.RoundResult == "fail" {
			failedNodes = append(failedNodes, fmt.Sprintf("%s/%s", r.NodeName, r.PodName))
			if r.FailedReason != nil && *r.FailedReason != "" {
				failedReason = *r.FailedReason
			}
		}

		switch {
		case r.TaskNetReach != nil:
			totalSucceed += r.TaskNetReach.SucceedCnt
			totalFailed += r.TaskNetReach.FailedCnt
			if r.TaskNetReach.LatencyP95Ms > latencyP95 {
				latencyP95 = r.TaskNetReach.LatencyP95Ms
			}
		case r.TaskAppHttpHealthy != nil:
			totalSucceed += r.TaskAppHttpHealthy.SucceedCnt
			totalFailed += r.TaskAppHttpHealthy.FailedCnt
			if r.TaskAppHttpHealthy.LatencyP95Ms > latencyP95 {
				latencyP95 = r.TaskAppHttpHealthy.LatencyP95Ms
			}
		case r.TaskNetDNS != nil:
			totalSucceed += r.TaskNetDNS.SucceedCnt
			totalFailed += r.TaskNetDNS.FailedCnt
		}
	}

	total := totalSucceed + totalFailed
	successRate := 0.0
	if total > 0 {
		successRate = float64(totalSucceed) / float64(total)
	}

	result := "succeed"
	if len(failedNodes) > 0 {
		result = "fail"
	}

	return &common.ProbeResult{
		TaskName:      taskName,
		TaskKind:      taskKind,
		RoundNumber:   roundNum,
		Result:        result,
		SuccessRate:   successRate,
		FailedNodes:   failedNodes,
		FailedReason:  failedReason,
		LatencyP95Ms:  latencyP95,
		RoundDuration: roundDur,
		Timestamp:     ts,
	}
}
