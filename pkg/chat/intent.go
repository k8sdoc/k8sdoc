package chat

import (
	"strings"
)

type Intent struct {
	Action    string
	Namespace string
	Resource  string
	Container string
	Filter    string
	LogLines  int64
	Raw       string
}

func ParseIntent(msg, currentNamespace string) Intent {
	lower := strings.ToLower(strings.TrimSpace(msg))
	intent := Intent{Raw: msg, Namespace: currentNamespace}

	if ns := extractNamespace(lower); ns != "" {
		intent.Namespace = ns
	}
	intent.Resource = extractResource(lower)


	if looksLikePodName(lower) {
		intent.Action = "describe_pod"
		intent.Resource = strings.TrimSpace(msg)
		return intent
	}

	switch {
	case matches(lower, "list pods", "show pods", "get pods", "what pods", "which pods",
		"pods are", "all pods", "running pods", "pods in", "pod names", "pod list",
		"show pod", "get pod", "give pods", "pods to select"):
		intent.Action = "list_pods"

	case matches(lower, "list deploy", "show deploy", "get deploy", "what deploy",
		"deployments", "deployment list", "show deployment", "all deployment"):
		intent.Action = "list_deployments"

	case matches(lower, "list service", "show service", "get service", "what service",
		"services", "svc", "show svc", "list svc"):
		intent.Action = "list_services"

	case matches(lower, "list node", "show node", "get node", "what node",
		"nodes", "cluster nodes", "all nodes"):
		intent.Action = "list_nodes"

	case matches(lower, "list namespace", "show namespace", "namespaces",
		"get ns", "list ns", "all namespace"):
		intent.Action = "list_namespaces"

	case matches(lower, "list pvc", "persistent volume", "storage claim", "pvcs", "pvc",
		"any volumes", "any pvc", "list volumes", "show volumes", "volumes in"):
		intent.Action = "list_pvcs"

	case matches(lower, "list configmap", "configmaps", "config map", "configmap"):
		intent.Action = "list_configmaps"

	case matches(lower, "list secret", "secrets", "show secrets"):
		intent.Action = "list_secrets"

	case matches(lower, "list ingress", "ingresses", "show ingress"):
		intent.Action = "list_ingress"

	case matches(lower, "list daemonset", "daemonsets", "show daemonset"):
		intent.Action = "list_daemonsets"

	case matches(lower, "list statefulset", "statefulsets", "show statefulset"):
		intent.Action = "list_statefulsets"

	case matches(lower, "list job", "jobs", "show jobs", "cronjob"):
		intent.Action = "list_jobs"

	case matches(lower, "list hpa", "horizontal pod", "autoscaler"):
		intent.Action = "list_hpa"

	case matches(lower, "events", "warnings", "recent events", "cluster events",
		"what happened", "warning events", "show events", "get events"):
		intent.Action = "list_events"
		if matches(lower, "warning", "error", "problem", "warn") {
			intent.Filter = "warnings"
		}

	case matches(lower, "summary", "overview", "cluster status", "cluster health",
		"how is the cluster", "what's running", "whats running", "health check"):
		intent.Action = "cluster_summary"

	case matches(lower, "logs", "log", "tail", "stdout", "stderr", "show log", "get log"):
		intent.Action = "get_logs"
		intent.LogLines = 100
		if matches(lower, "50 lines", "last 50") {
			intent.LogLines = 50
		}
		if matches(lower, "200 lines", "last 200") {
			intent.LogLines = 200
		}
		if matches(lower, "500 lines", "last 500") {
			intent.LogLines = 500
		}

	case matches(lower, "describe pod", "detail pod", "pod detail", "info about pod",
		"inspect pod", "inspect the pod", "look at pod", "check pod",
		"show me pod", "tell me about pod", "inspect metrics", "inspect coredns",
		"inspect traefik", "inspect kdoctor", "i said this",
		"full configuration", "full config", "configuration of", "get config",
		"show config", "yaml of", "spec of", "pod spec",
		"inspect that", "inspect configuration", "inspect the",
		"configuration of that", "config of that", "show that pod",
		"details of that", "detail of that", "info of that"):
		intent.Action = "describe_pod"

	case matches(lower, "describe node", "detail node", "node detail"):
		intent.Action = "describe_node"

	case matches(lower, "describe deployment", "detail deployment"):
		intent.Action = "describe_deployment"

	case matches(lower, "describe service", "detail service"):
		intent.Action = "describe_service"

	case matches(lower, "check my yaml", "lint yaml", "validate yaml",
		"analyze yaml", "check manifest", "lint manifest", "validate manifest",
		"check config", "analyze config", "lint config",
		"check my deployment", "check my service", "check my ingress",
		"what's wrong with my yaml", "whats wrong with my yaml"):
		intent.Action = "lint_manifest"

	case matches(lower, "security scan", "security check", "security audit",
		"check security", "privileged", "root container", "security issues",
		"rbac", "vulnerability", "cve", "secure", "hardening",
		"running as root", "run as root", "runas", "runasroot",
		"no security context", "security context", "securitycontext",
		"check for containers", "containers running", "root access",
		"allowprivilegeescalation", "privilege escalation", "non-root",
		"read only filesystem", "readonly filesystem",
		"running with root", "root privilege", "with root", "root user",
		"which pods root", "pods running root", "containers root",
		"who is root", "which are root", "which all are running",
		"running with privilege", "root permissions"):
		intent.Action = "security_scan"

	case matches(lower, "no resource limits", "missing resource limits", "no limits",
		"resource limits", "containers with no", "no cpu limit", "no memory limit",
		"unlimited cpu", "unlimited memory", "without limits", "check limits",
		"find containers", "containers without"):
		intent.Action = "resource_limits"

	case matches(lower, "resource usage", "cpu usage", "memory usage", "top pods",
		"top nodes", "resource consumption", "how much cpu", "how much memory"):
		intent.Action = "resource_usage"

	case matches(lower, "network connectivity", "check network", "dns status",
		"check dns", "network status", "connectivity status",
		"can pods communicate", "network policy", "reachability",
		"check connectivity", "test network", "network check"):
		intent.Action = "network_diag"

	case matches(lower, "is there any", "is there a ", "are there any",
		"still there", "still exist", "check if", "check again",
		"does it exist", "do they exist", "is it still", "are they still",
		"verify", "confirm", "check whether"):

		if matches(lower, "error", "issue", "problem", "fault", "failure", "warning",
			"wrong", "failing", "broken", "crash") {
			intent.Action = "diagnose"
			break
		}

		switch {
		case matches(lower, "volume", "pvc", "persistent"):
			intent.Action = "list_pvcs"
		case matches(lower, "deploy", "deployment"):
			intent.Action = "list_deployments"
		case matches(lower, "service", "svc"):
			intent.Action = "list_services"
		case matches(lower, "secret"):
			intent.Action = "list_secrets"
		case matches(lower, "node"):
			intent.Action = "list_nodes"
		case matches(lower, "namespace"):
			intent.Action = "list_namespaces"
		default:
			intent.Action = "list_pods"
		}

	case matches(lower, "why is", "why are", "why isn't", "why is not",
		"what's wrong", "whats wrong", "what is wrong",
		"why failing", "why failed", "why crash", "why error",
		"crashing", "not working", "not starting", "not running",
		"troubleshoot", "debug", "diagnose", "fix",
		"pending", "error", "issue", "problem", "oomkilled",
		"crashloop", "imagepull", "evicted"):
		intent.Action = "diagnose"

	case matches(lower, "analyze all", "analyse all", "full scan", "find all issues",
		"detect problems", "full analysis", "run analysis", "scan cluster",
		"check everything", "full diagnostic"):
		intent.Action = "analyze_all"

	case matches(lower, "what is kubernetes", "what is k8s", "explain kubernetes",
		"what is a pod", "what is a deployment", "what is a service", "what is a pvc",
		"what is a configmap", "what is a secret", "what is an ingress",
		"what is a namespace", "what is a node", "what is a daemonset",
		"what is a statefulset", "what is a job", "what is hpa",
		"how does", "how do i", "how to", "how can i", "how do you",
		"what does", "explain ", "tell me about",
		"kubectl cheat", "kubectl command", "cheat sheet",
		"best practice", "difference between", "when to use",
		"inspect volume", "exec into", "copy file", "port forward",
		"what are", "i want to"):
		intent.Action = "knowledge"

	case matches(lower, "switch to namespace", "use namespace",
		"change namespace", "set namespace"):
		intent.Action = "switch_namespace"

	case matches(lower, "rollout", "rollback", "deployment history", "roll back"):
		intent.Action = "rollout_status"

	default:
		intent.Action = "chat"
	}

	return intent
}

func matches(text string, patterns ...string) bool {
	for _, p := range patterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}

func extractNamespace(text string) string {
	patterns := []string{"in namespace ", "-n ", "--namespace "}
	for _, p := range patterns {
		if idx := strings.Index(text, p); idx != -1 {
			rest := text[idx+len(p):]
			parts := strings.Fields(rest)
			if len(parts) > 0 {
				ns := strings.Trim(parts[0], ".,;?!")
				if ns != "" && !isKeyword(ns) && !isNonNamespace(ns) {
					return ns
				}
			}
		}
	}
	return ""
}


func isNonNamespace(s string) bool {
	non := map[string]bool{
		"status": true, "check": true, "connectivity": true, "dns": true,
		"health": true, "running": true, "failing": true, "pending": true,
		"error": true, "issue": true, "problem": true, "cluster": true,
		"pod": true, "node": true, "service": true, "deploy": true,
	}
	return non[strings.ToLower(s)]
}

func extractResource(text string) string {
	markers := []string{
		"logs of ", "log of ", "logs from ", "log from ",
		"logs for ", "log for ", "get logs of ", "get logs from ", "get logs for ",
		"show logs of ", "show logs from ", "show logs for ",
		"pod ", "deployment ", "service ", "node ", "pvc ", "svc ",
		"ingress ", "daemonset ", "statefulset ", "job ", "cronjob ",
		"describe ", "diagnose ",
	}
	for _, m := range markers {
		if idx := strings.Index(text, m); idx != -1 {
			rest := text[idx+len(m):]
			parts := strings.Fields(rest)
			if len(parts) > 0 {
				name := strings.Trim(parts[0], ".,;?!\"'")
				if !isKeyword(name) && len(name) > 1 {
					return name
				}
			}
		}
	}

	words := strings.Fields(text)
	for _, w := range words {
		w = strings.Trim(w, ".,;?!\"'")
		if strings.Count(w, "-") >= 1 && len(w) > 4 && !isKeyword(w) {
			// Looks like a kubernetes resource name (has hyphens, not a keyword)
			skipWords := []string{"crash-loop", "image-pull", "not-ready", "non-root",
				"step-by-step", "well-known", "read-only", "write-only"}
			skip := false
			for _, s := range skipWords {
				if w == s { skip = true; break }
			}
			if !skip {
				return w
			}
		}
	}
	return ""
}

func isKeyword(s string) bool {
	kw := map[string]bool{
		"in": true, "all": true, "the": true, "a": true, "an": true,
		"is": true, "are": true, "be": true, "running": true,
		"pending": true, "failing": true, "list": true, "show": true,
		"my": true, "this": true, "that": true, "which": true,
	}
	return kw[strings.ToLower(s)]
}


func isContextualRef(msg string) bool {
	lower := strings.ToLower(msg)
	contextWords := []string{
		"that", "it", "the pod", "same", "this pod", "this one",
		"that one", "its logs", "those logs", "the nginx", "the app",
		"that nginx", "that crash", "the crash", "the broken",
		"it's", "its", "the same",
	}
	for _, w := range contextWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}


func looksLikePodName(s string) bool {
	s = strings.TrimSpace(s)

	if strings.Contains(s, " ") || len(s) < 3 || len(s) > 253 {
		return false
	}

	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}

	return strings.Contains(s, "-")
}

