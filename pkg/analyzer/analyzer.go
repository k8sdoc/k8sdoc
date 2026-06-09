// Package analyzer contains all k8sdoc analyzer implementations.
// Each analyzer implements common.IAnalyzer and focuses on one resource kind
package analyzer

import (
	"github.com/user/k8sdoc/pkg/common"
)

// It contains analyzers always enabled by default
var coreAnalyzerMap = map[string]common.IAnalyzer{
	"Pod":        PodAnalyzer{},
	"Deployment": DeploymentAnalyzer{},
	"Service":    ServiceAnalyzer{},
	"Node":       NodeAnalyzer{},
}

// It contains analyzers for kdoctor CRD results
// These are automatically enabled when kdoctor CRDs are present
var kdoctorAnalyzerMap = map[string]common.IAnalyzer{
	"NetReach":       NetReachAnalyzer{},
	"AppHttpHealthy": AppHttpHealthyAnalyzer{},
	"Netdns":         NetDNSAnalyzer{},
}


var additionalAnalyzerMap = map[string]common.IAnalyzer{
	"PersistentVolumeClaim": PVCAnalyzer{},
	"Ingress":               IngressAnalyzer{},
}

func ListFilters() (core, kdoctor, additional []string) {
	for k := range coreAnalyzerMap {
		core = append(core, k)
	}
	for k := range kdoctorAnalyzerMap {
		kdoctor = append(kdoctor, k)
	}
	for k := range additionalAnalyzerMap {
		additional = append(additional, k)
	}
	return
}

func GetAnalyzers(filters []string) map[string]common.IAnalyzer {
	if len(filters) == 0 {
		merged := make(map[string]common.IAnalyzer)
		for k, v := range coreAnalyzerMap {
			merged[k] = v
		}
		for k, v := range kdoctorAnalyzerMap {
			merged[k] = v
		}
		return merged
	}

	result := make(map[string]common.IAnalyzer)
	all := make(map[string]common.IAnalyzer)
	for k, v := range coreAnalyzerMap {
		all[k] = v
	}
	for k, v := range kdoctorAnalyzerMap {
		all[k] = v
	}
	for k, v := range additionalAnalyzerMap {
		all[k] = v
	}
	for _, f := range filters {
		if a, ok := all[f]; ok {
			result[f] = a
		}
	}
	return result
}
