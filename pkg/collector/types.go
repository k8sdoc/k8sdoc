package collector

import (
	"time"

	"github.com/user/k8sdoc/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


type EnrichedContext struct {
	ResourceFailures []common.Result      `json:"resourceFailures"`
	ProbeResults     []common.ProbeResult `json:"probeResults"`
	ClusterName      string               `json:"clusterName,omitempty"`
	Namespace        string               `json:"namespace"`
	Timestamp        time.Time            `json:"timestamp"`
}


type TaskStatus struct {
	ExpectedRound   *int64       `json:"expectedRound,omitempty"`
	DoneRound       *int64       `json:"doneRound,omitempty"`
	Finish          bool         `json:"finish"`
	FinishTime      *metav1.Time `json:"finishTime,omitempty"`
	LastRoundStatus *string      `json:"lastRoundStatus,omitempty"`
}

type KdoctorReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Report            KdoctorReports `json:"report,omitempty"`
	Status            KdoctorStatus  `json:"status,omitempty"`
	Task              TaskInfo       `json:"task,omitempty"`
}

type KdoctorReports struct {
	LatestRoundReport *[]RoundReport `json:"latestRoundReport,omitempty"`
}

type KdoctorStatus struct {
	TotalRoundNumber    int64  `json:"totalRoundNumber"`
	FinishedRoundNumber int64  `json:"roundFinishedNumber"`
	Status              string `json:"status"`
	RoundNumber         int64  `json:"roundNumber"`
}

type TaskInfo struct {
	TaskName string `json:"name"`
	TaskType string `json:"kind"`
}

type RoundReport struct {
	RoundNumber    int64       `json:"roundNumber"`
	RoundResult    string      `json:"roundResult"` // succeed | fail
	NodeName       string      `json:"nodeName"`
	PodName        string      `json:"podName"`
	FailedReason   *string     `json:"reasonsForFailure,omitempty"`
	StartTimeStamp metav1.Time `json:"roundStartTimeStamp"`
	EndTimeStamp   metav1.Time `json:"roundEndTimeStamp"`
	RoundDuration  string      `json:"roundDuration"`

	TaskNetReach       *NetReachDetail       `json:"taskNetReach,omitempty"`
	TaskAppHttpHealthy *AppHttpHealthyDetail `json:"taskAppHealthy,omitempty"`
	TaskNetDNS         *NetDNSDetail         `json:"taskNetDns,omitempty"`
}

type NetReachDetail struct {
	SucceedCnt   int64   `json:"succeedCnt"`
	FailedCnt    int64   `json:"failedCnt"`
	TotalCnt     int64   `json:"totalCnt"`
	SuccessRate  float64 `json:"successRate"`
	LatencyP95Ms int64   `json:"latencyP95Ms,omitempty"`
}

type AppHttpHealthyDetail struct {
	SucceedCnt   int64   `json:"succeedCnt"`
	FailedCnt    int64   `json:"failedCnt"`
	TotalCnt     int64   `json:"totalCnt"`
	SuccessRate  float64 `json:"successRate"`
	HTTP5xxCnt   int64   `json:"http5xxCnt,omitempty"`
	TimeoutCnt   int64   `json:"timeoutCnt,omitempty"`
	LatencyP95Ms int64   `json:"latencyP95Ms,omitempty"`
}

type NetDNSDetail struct {
	SucceedCnt  int64   `json:"succeedCnt"`
	FailedCnt   int64   `json:"failedCnt"`
	TotalCnt    int64   `json:"totalCnt"`
	SuccessRate float64 `json:"successRate"`
	NXDomainCnt int64   `json:"nxdomainCnt,omitempty"`
	TimeoutCnt  int64   `json:"timeoutCnt,omitempty"`
}
