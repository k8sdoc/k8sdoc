package chat

import (
	"context"
	"fmt"
	"strings"
)

type FetchedData struct {
	Tables      map[string]*TableData // keyed by data source name
	TextData    map[string]string     // plain text versions
	PodDescribe string                // for describe_pod
	Logs        string                // for get_logs
	PodName     string                // resolved full pod name
	PodNS       string                // resolved pod namespace
}

func (e *Engine) Fetch(ctx context.Context, cls *Classification, session *Session) *FetchedData {
	fd := &FetchedData{
		Tables:   make(map[string]*TableData),
		TextData: make(map[string]string),
	}

	ns := cls.Namespace
	if ns != "" && !isValidNamespaceName(ns) {
		ns = ""
	}

	sources := cls.DataSources
	if len(sources) == 0 {
		sources = DataSourcesForIntent(cls.Intent)
	}

	podName := cls.ResourceName
	if podName == "" {
		podName = session.LastResource
	}

	for _, src := range sources {
		switch src {
		case "pods":
			tbl, txt, err := e.k8s.ListPods(ctx, "", "")
			if err == nil {
				fd.Tables["pods"] = tbl
				fd.TextData["pods"] = txt
			}

		case "deployments":
			tbl, txt, err := e.k8s.ListDeployments(ctx, "")
			if err == nil {
				fd.Tables["deployments"] = tbl
				fd.TextData["deployments"] = txt
			}

		case "services":
			tbl, txt, err := e.k8s.ListServices(ctx, "")
			if err == nil {
				fd.Tables["services"] = tbl
				fd.TextData["services"] = txt
			}

		case "nodes":
			tbl, txt, err := e.k8s.ListNodes(ctx)
			if err == nil {
				fd.Tables["nodes"] = tbl
				fd.TextData["nodes"] = txt
			}

		case "namespaces":
			tbl, txt, err := e.k8s.ListNamespaces(ctx)
			if err == nil {
				fd.Tables["namespaces"] = tbl
				fd.TextData["namespaces"] = txt
			}

		case "events":
			tbl, txt, err := e.k8s.ListEvents(ctx, "", true)
			if err == nil {
				fd.Tables["events"] = tbl
				fd.TextData["events"] = txt
			}

		case "secrets":
			tbl, txt, err := e.k8s.ListSecrets(ctx, "")
			if err == nil {
				fd.Tables["secrets"] = tbl
				fd.TextData["secrets"] = txt
			}

		case "configmaps":
			tbl, txt, err := e.k8s.ListConfigMaps(ctx, "")
			if err == nil {
				fd.Tables["configmaps"] = tbl
				fd.TextData["configmaps"] = txt
			}

		case "pvcs":
			tbl, txt, err := e.k8s.ListPVCs(ctx, "")
			if err == nil {
				fd.Tables["pvcs"] = tbl
				fd.TextData["pvcs"] = txt
			}
			pvTbl, pvTxt, err := e.k8s.ListPersistentVolumes(ctx)
			if err == nil {
				fd.Tables["pvs"] = pvTbl
				fd.TextData["pvs"] = pvTxt
			}

		case "ingresses":
			tbl, txt, err := e.k8s.ListIngresses(ctx, "")
			if err == nil {
				fd.Tables["ingresses"] = tbl
				fd.TextData["ingresses"] = txt
			}

		case "daemonsets":
			tbl, txt, err := e.k8s.ListDaemonSets(ctx, "")
			if err == nil {
				fd.Tables["daemonsets"] = tbl
				fd.TextData["daemonsets"] = txt
			}

		case "statefulsets":
			tbl, txt, err := e.k8s.ListStatefulSets(ctx, "")
			if err == nil {
				fd.Tables["statefulsets"] = tbl
				fd.TextData["statefulsets"] = txt
			}

		case "jobs":
			tbl, txt, err := e.k8s.ListJobs(ctx, "")
			if err == nil {
				fd.Tables["jobs"] = tbl
				fd.TextData["jobs"] = txt
			}

		case "pod_detail":
			candidates := []string{}
			if cls.ResourceName != "" { candidates = append(candidates, cls.ResourceName) }
			if session.LastResource != "" && session.LastResource != cls.ResourceName {
				candidates = append(candidates, session.LastResource)
			}
			if podName != "" && podName != cls.ResourceName {
				candidates = append(candidates, podName)
			}
			for _, candidate := range candidates {
				full, resolvedNS := e.k8s.ResolvePodName(ctx, ns, candidate)
				if full != "" {
					fd.PodName = full
					fd.PodNS = resolvedNS
					desc, err := e.k8s.DescribePod(ctx, resolvedNS, full)
					if err == nil {
						fd.PodDescribe = desc
						fd.TextData["pod_detail"] = desc
					}
					break
				}
			}

		case "logs":
			logCandidates := []string{}
			if cls.ResourceName != "" { logCandidates = append(logCandidates, cls.ResourceName) }
			if session.LastResource != "" { logCandidates = append(logCandidates, session.LastResource) }
			if podName != "" { logCandidates = append(logCandidates, podName) }
			if len(logCandidates) == 0 { break }
			podName = logCandidates[0]
			if podName != "" {
				full, resolvedNS := e.k8s.ResolvePodName(ctx, ns, podName)
				if full != "" {
					fd.PodName = full
					fd.PodNS = resolvedNS
					lines := cls.LogLines
					if lines == 0 {
						lines = 100
					}
					logs, err := e.k8s.GetPodLogs(ctx, resolvedNS, full, "", lines)
					if err == nil {
						fd.Logs = logs
						fd.TextData["logs"] = logs
					} else {
						fd.TextData["logs_error"] = fmt.Sprintf("Could not fetch logs for %s/%s: %v", resolvedNS, full, err)
					}
				} else {
					fd.TextData["logs_error"] = fmt.Sprintf("Could not find pod matching %q", podName)
				}
			}
		}
	}

	return fd
}

func (fd *FetchedData) PrimaryTable(intent string) *TableData {
	primary := map[string]string{
		"list_pods":        "pods",
		"list_deployments": "deployments",
		"list_services":    "services",
		"list_nodes":       "nodes",
		"list_namespaces":  "namespaces",
		"list_events":      "events",
		"list_secrets":     "secrets",
		"list_configmaps":  "configmaps",
		"list_pvcs":        "pvcs",
		"list_ingress":     "ingresses",
		"list_daemonsets":  "daemonsets",
		"list_statefulsets":"statefulsets",
		"list_jobs":        "jobs",
		"diagnose":         "pods",
		"cluster_summary":  "pods",
	}
	if key, ok := primary[intent]; ok {
		return fd.Tables[key]
	}
	for _, tbl := range fd.Tables {
		return tbl
	}
	return nil
}

func (fd *FetchedData) AllText() string {
	var parts []string

	order := []string{
		"pod_detail", "logs", "pods", "deployments", "services",
		"nodes", "events", "namespaces", "secrets", "configmaps",
		"pvcs", "pvs", "ingresses", "daemonsets", "statefulsets", "jobs",
	}

	for _, key := range order {
		if txt, ok := fd.TextData[key]; ok && txt != "" {
			label := strings.ToUpper(key)
			parts = append(parts, fmt.Sprintf("=== %s ===\n%s", label, txt))
		}
	}

	if errTxt, ok := fd.TextData["logs_error"]; ok {
		parts = append(parts, "=== ERROR ===\n"+errTxt)
	}

	return strings.Join(parts, "\n\n")
}


func (fd *FetchedData) ExistenceCheck(intent, filterTerm string) (bool, []string) {
	tbl := fd.PrimaryTable(intent)
	if tbl == nil || filterTerm == "" {
		return false, nil
	}
	filter := strings.ToLower(filterTerm)
	var matched []string
	seen := map[string]bool{}
	for _, row := range tbl.Rows {
		for _, v := range row {
			if strings.Contains(strings.ToLower(v), filter) {
				if name, ok := row["NAME"]; ok && !seen[name] {
					matched = append(matched, name)
					seen[name] = true
				}
				break
			}
		}
	}
	return len(matched) > 0, matched
}
