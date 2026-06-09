package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/user/k8sdoc/pkg/ai"
)

// Classification is the structured output of Stage 1.
// The classifier LLM returns this as JSON — never hallucinating cluster data,
// just deciding WHAT to fetch and HOW to answer.
type Classification struct {
	Intent string `json:"intent"`

	// ResourceType is what kind of K8s resource the user is asking about
	// e.g. "pod", "deployment", "service", "node", "pvc", "secret" etc.
	ResourceType string `json:"resourceType,omitempty"`

	// ResourceName is a specific resource name or partial name
	// e.g. "metrics-server", "nginx-crash", "coredns"
	ResourceName string `json:"resourceName,omitempty"`

	// Namespace to scope the query (empty = all namespaces)
	Namespace string `json:"namespace,omitempty"`

	// FilterTerm is a search term for existence checks
	// e.g. "mysql", "redis", "nginx"
	FilterTerm string `json:"filterTerm,omitempty"`

	// IsExistenceCheck is true when user is asking if something exists
	IsExistenceCheck bool `json:"isExistenceCheck,omitempty"`

	// NeedsLiveData is false for pure knowledge/howto questions
	NeedsLiveData bool `json:"needsLiveData"`

	// DataSources lists what cluster data to fetch
	// e.g. ["pods", "events", "deployments"]
	DataSources []string `json:"dataSources,omitempty"`

	// DirectAnswer is set when classifier can answer without cluster data
	// e.g. for "what is kubernetes" or "how to delete a pod"
	DirectAnswer string `json:"directAnswer,omitempty"`

	// AnswerStyle controls how Stage 2 should respond
	// "diagnose" | "summarise" | "explain" | "describe" | "list" | "yesno"
	AnswerStyle string `json:"answerStyle"`

	// LogLines for log fetch requests
	LogLines int64 `json:"logLines,omitempty"`
}

const classifierSystemPrompt = `You are a Kubernetes query classifier. Your job is to analyze user messages and return a JSON classification object. You do NOT answer questions — you only classify what data is needed and how to answer.

IMPORTANT: A very wide range of questions should be classified as "knowledge" with needsLiveData=false:
- Conceptual: "what is X", "explain X", "how does X work", "what are X"
- How-to: "how to X", "how do I X", "how can I X", "steps to X"  
- Commands: "kubectl command for X", "how to delete X", "how to scale X"
- Troubleshooting concepts: "what causes OOMKilled", "what is CrashLoopBackOff"
- Best practices: "should I use X", "difference between X and Y", "when to use X"
- Architecture: "how does networking work", "what is CNI", "explain etcd"
- Any question about Kubernetes concepts, YAML structure, kubectl syntax
Only classify as needsLiveData=true when the user specifically wants to see THEIR cluster's actual state.

VALID INTENTS:
- list_pods        — user wants to see pods
- list_deployments — user wants to see deployments  
- list_services    — user wants to see services
- list_nodes       — user wants to see nodes
- list_namespaces  — user wants to see namespaces
- list_secrets     — user wants to see secrets
- list_configmaps  — user wants to see configmaps
- list_pvcs        — user wants to see PVCs/volumes
- list_ingress     — user wants to see ingresses
- list_daemonsets  — user wants to see daemonsets
- list_statefulsets— user wants to see statefulsets
- list_jobs        — user wants to see jobs
- list_events      — user wants to see events/warnings
- get_logs         — user wants pod logs
- describe_pod     — user wants detailed pod info or configuration
- describe_node    — user wants node details
- diagnose         — user wants to know what's wrong / why something is failing
- analyze_all      — user wants full cluster health scan
- security_scan    — user wants security/privilege check
- resource_limits  — user wants to find containers without resource limits
- network_diag     — user wants network/DNS/connectivity info
- cluster_summary  — user wants overall cluster health
- knowledge        — general K8s question, no cluster data needed
- manifest_lint    — user wants to lint/check a YAML file
- switch_namespace — user wants to change namespace context
- rollout_status   — user wants deployment rollout info
- exec_command     — user wants to run a kubectl command

ANSWER STYLES:
- "yesno"     — for existence questions ("are there any X", "is there a Y")
- "summarise" — for list requests (show table + 1 sentence)
- "diagnose"  — for error/problem questions (Problem → Root Cause → Fix)
- "describe"  — for detail/configuration requests
- "explain"   — for general knowledge questions
- "logs"      — for log analysis

DATA SOURCES (what to fetch):
pods, deployments, services, nodes, namespaces, events, secrets, configmaps,
pvcs, ingresses, daemonsets, statefulsets, jobs, logs, node_detail, pod_detail

RULES:
1. NeedsLiveData=false ONLY for pure knowledge questions (what is X, how to Y, explain Z)
2. For "are there any X pods" → intent=list_pods, filterTerm="x", isExistenceCheck=true, answerStyle="yesno"
3. For "why is X failing" → intent=diagnose, resourceName="x", dataSources=["pods","events"]
4. For "show me logs of X" → intent=get_logs, resourceName="x", logLines=100
5. For "inspect/describe/config of X" → intent=describe_pod, resourceName="x", answerStyle="describe"
6. Never put cluster data in directAnswer — only factual K8s knowledge
7. For ambiguous queries, prefer fetching data over guessing
8. Extract resourceName from natural language: "metrics server"→"metrics-server", "core dns"→"coredns", "nginix"→"nginx", "kube dns"→"coredns"
9. Namespace: only set if explicitly mentioned. Empty = all namespaces.
10. "show/list/get pods running" → intent=list_pods, needsLiveData=true
11. Handle typos: "nginix"→"nginx", "deployement"→"deployment", "mesage"→"message"
12. When user says "that pod", "it", "same pod" → use the last resource from session context
13. NEVER generate kubectl commands with <placeholder> — always use real names or omit

Respond with ONLY valid JSON. No explanation, no markdown, no extra text.`

// Classify sends the user message to Stage 1 LLM for structured intent classification
func Classify(ctx context.Context, aiClient ai.IAI, session *Session, userMsg string) (*Classification, error) {
	// Build context from recent conversation (last 4 turns for efficiency)
	var historyLines []string
	history := session.Messages
	if len(history) > 8 {
		history = history[len(history)-8:]
	}
	for _, m := range history {
		if m.Role == "user" || m.Role == "assistant" {
			role := "User"
			if m.Role == "assistant" {
				role = "Assistant"
			}
			content := m.Content
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			historyLines = append(historyLines, fmt.Sprintf("%s: %s", role, content))
		}
	}

	contextBlock := ""
	if len(historyLines) > 0 {
		contextBlock = fmt.Sprintf("\nRecent conversation:\n%s\n", strings.Join(historyLines, "\n"))
	}

	sessionInfo := fmt.Sprintf(
		"Current namespace: %q. Last pod/resource seen: %q (namespace: %q).",
		session.Namespace, session.LastResource, session.LastResourceNS)

	// Build explicit context hint for "that X" references
	contextHint := ""
	if session.LastResource != "" {
		contextHint = fmt.Sprintf("\nIMPORTANT: The user previously saw pod %q in namespace %q. If they say \"that pod\", \"it\", \"that nginx\", or similar, set resourceName=%q.",
			session.LastResource, session.LastResourceNS, session.LastResource)
	}

	prompt := fmt.Sprintf(`%s%s

Session: %s%s

Classify this user message and return JSON:
"%s"

Return ONLY JSON, nothing else.`, contextBlock, "", sessionInfo, contextHint, userMsg)

	response, err := aiClient.GetChatCompletion(ctx, []ai.ChatMessage{
		{Role: "system", Content: classifierSystemPrompt},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, fmt.Errorf("classifier error: %w", err)
	}

	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var cls Classification
	if err := json.Unmarshal([]byte(response), &cls); err != nil {
		if start := strings.Index(response, "{"); start != -1 {
			if end := strings.LastIndex(response, "}"); end > start {
				if err2 := json.Unmarshal([]byte(response[start:end+1]), &cls); err2 != nil {
					return fallbackClassify(userMsg, session), nil
				}
				return &cls, nil
			}
		}
		return fallbackClassify(userMsg, session), nil
	}
	return &cls, nil
}

func fallbackClassify(userMsg string, session *Session) *Classification {
	intent := ParseIntent(userMsg, session.Namespace)
	cls := &Classification{
		Intent:        intent.Action,
		ResourceName:  intent.Resource,
		Namespace:     intent.Namespace,
		NeedsLiveData: true,
		AnswerStyle:   "summarise",
		LogLines:      intent.LogLines,
	}
	if intent.LogLines > 0 {
		cls.DataSources = []string{"logs"}
		cls.AnswerStyle = "logs"
	}
	return cls
}

func DataSourcesForIntent(intent string) []string {
	switch intent {
	case "list_pods":
		return []string{"pods"}
	case "list_deployments":
		return []string{"deployments"}
	case "list_services":
		return []string{"services"}
	case "list_nodes":
		return []string{"nodes"}
	case "list_namespaces":
		return []string{"namespaces"}
	case "list_secrets":
		return []string{"secrets"}
	case "list_configmaps":
		return []string{"configmaps"}
	case "list_pvcs":
		return []string{"pvcs"}
	case "list_ingress":
		return []string{"ingresses"}
	case "list_daemonsets":
		return []string{"daemonsets"}
	case "list_statefulsets":
		return []string{"statefulsets"}
	case "list_jobs":
		return []string{"jobs"}
	case "list_events":
		return []string{"events"}
	case "get_logs":
		return []string{"logs"}
	case "describe_pod":
		return []string{"pod_detail"}
	case "diagnose":
		return []string{"pods", "events", "deployments"}
	case "analyze_all":
		return []string{"pods", "events", "deployments", "services", "nodes"}
	case "security_scan":
		return []string{"pods"}
	case "resource_limits":
		return []string{"pods"}
	case "network_diag":
		return []string{"services", "events", "pods"}
	case "cluster_summary":
		return []string{"pods", "nodes", "deployments", "namespaces"}
	case "rollout_status":
		return []string{"deployments"}
	default:
		return []string{"pods"}
	}
}
