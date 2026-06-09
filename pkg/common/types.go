package common

import (
	"context"
	"time"

	"github.com/user/k8sdoc/pkg/kubernetes"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
)

type IAnalyzer interface {
	Analyze(a Analyzer) ([]Result, error)
}

type Analyzer struct {
	Client        *kubernetes.Client
	Context       context.Context
	Namespace     string
	LabelSelector string
	PreAnalysis   map[string]PreAnalysis
	Results       []Result
	ProbeResults  []ProbeResult
}

type PreAnalysis struct {
	Pod            v1.Pod
	FailureDetails []Failure
	Deployment     appsv1.Deployment
	ReplicaSet     appsv1.ReplicaSet
	DaemonSet      appsv1.DaemonSet
	StatefulSet    appsv1.StatefulSet
	Service        v1.Service
	Endpoint       v1.Endpoints
	Ingress        networkv1.Ingress
	Node           v1.Node
	PVC            v1.PersistentVolumeClaim
}

type Result struct {
	Kind         string    `json:"kind"`
	Name         string    `json:"name"`
	Error        []Failure `json:"error"`
	Details      string    `json:"details,omitempty"`
	ParentObject string    `json:"parentObject,omitempty"`
	ProbeRef     string    `json:"probeRef,omitempty"`
}

type Failure struct {
	Text      string
	Sensitive []Sensitive
}

type Sensitive struct {
	Unmasked string
	Masked   string
}

type ProbeResult struct {
	TaskName      string    `json:"taskName"`
	TaskKind      string    `json:"taskKind"`
	RoundNumber   int64     `json:"roundNumber"`
	Result        string    `json:"result"`
	SuccessRate   float64   `json:"successRate"`
	FailedNodes   []string  `json:"failedNodes,omitempty"`
	FailedReason  string    `json:"failedReason,omitempty"`
	LatencyP95Ms  int64     `json:"latencyP95Ms,omitempty"`
	RoundDuration string    `json:"roundDuration,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}

type Diagnosis struct {
	ResourceKind       string   `json:"resourceKind"`
	ResourceName       string   `json:"resourceName"`
	RootCause          string   `json:"rootCause"`
	AffectedComponents []string `json:"affectedComponents,omitempty"`
	SolutionSteps      []string `json:"solutionSteps"`
	Confidence         string   `json:"confidence"`
	ProbeEvidence      string   `json:"probeEvidence,omitempty"`
}

type AnalysisStatus string

const (
	StateOK              AnalysisStatus = "OK"
	StateProblemDetected AnalysisStatus = "ProblemDetected"
)
