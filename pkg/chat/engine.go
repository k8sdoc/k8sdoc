package chat

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/user/k8sdoc/pkg/ai"
	"github.com/user/k8sdoc/pkg/analysis"
	"github.com/user/k8sdoc/pkg/diagnostic"
	"github.com/user/k8sdoc/pkg/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Engine is the two-stage conversational diagnostic engine

// Stage 1 (Classifier): Reads the user message and recent history, returns JSON describing what data to fetch and how to answer
// Stage 2 (Responder): Streams the answer using fetched live cluster data as ground truth. Never hallucinates because data is always pre-fetched
type Engine struct {
	aiClient  ai.IAI
	k8s       *K8sTools
	client    *kubernetes.Client
	executor  *Executor
	sessions  map[string]*Session
	mu        sync.RWMutex
	namespace string
}

func NewEngine(aiClient ai.IAI, client *kubernetes.Client, defaultNamespace string) *Engine {
	return &Engine{
		aiClient:  aiClient,
		k8s:       NewK8sTools(client),
		client:    client,
		executor:  NewExecutor(client),
		sessions:  make(map[string]*Session),
		namespace: defaultNamespace,
	}
}

func (e *Engine) GetOrCreateSession(sessionID string) *Session {
	e.mu.Lock()
	defer e.mu.Unlock()
	if s, ok := e.sessions[sessionID]; ok {
		return s
	}
	s := &Session{
		ID:        sessionID,
		Namespace: e.namespace,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	e.sessions[sessionID] = s
	return s
}


func (e *Engine) Chat(ctx context.Context, sessionID, userMsg string, onChunk func(StreamChunk)) {
	session := e.GetOrCreateSession(sessionID)
	e.addMessage(session, "user", userMsg, nil, "")

	// Detect kubectl error output pasted into chat
	if strings.Contains(userMsg, "error:") && strings.Contains(strings.ToLower(userMsg), "kubectl") {
		e.streamWithData(ctx, session,
			"The user pasted this kubectl error. Explain what it means and give the correct syntax: "+userMsg,
			"", "explain", onChunk)
		return
	}

	// Handle action button responses directly
	if strings.HasPrefix(userMsg, "__exec__:") {
		e.executeCommand(ctx, session, strings.TrimPrefix(userMsg, "__exec__:"), onChunk)
		return
	}
	if userMsg == "__skip__" {
		onChunk(StreamChunk{Type: "token", Content: "Skipped."})
		onChunk(StreamChunk{Type: "done"})
		return
	}

	onChunk(StreamChunk{Type: "token", Content: ""}) // keeps connection alive

	cls, err := Classify(ctx, e.aiClient, session, userMsg)
	if err != nil {
		cls = fallbackClassify(userMsg, session)
	}

	if !cls.NeedsLiveData {
		fb := fallbackClassify(userMsg, session)
		if fb.Intent == "describe_pod" || fb.Intent == "get_logs" || fb.Intent == "diagnose" || fb.Intent == "network_diag" {
			cls = fb
		}
	}

	// Override: "what version of docker/runtime/kubelet" -> fetch node info
	if strings.Contains(strings.ToLower(userMsg), "version of docker") ||
		strings.Contains(strings.ToLower(userMsg), "container runtime") ||
		strings.Contains(strings.ToLower(userMsg), "docker version") ||
		strings.Contains(strings.ToLower(userMsg), "kubelet version") {
		runtimeInfo, err := e.k8s.GetContainerRuntimeInfo(ctx)
		if err == nil && runtimeInfo != "" {
			onChunk(StreamChunk{Type: "data", Data: runtimeInfo, DataType: "yaml"})
			e.streamWithData(ctx, session, userMsg, runtimeInfo, "explain", onChunk)
			return
		}
	}

	// Override: force existence check for "is there any X" / "are there any X" patterns
	if cls.FilterTerm == "" || !cls.IsExistenceCheck {
		msgLower := strings.ToLower(userMsg)
		if strings.Contains(msgLower, "is there any") || strings.Contains(msgLower, "are there any") ||
			strings.Contains(msgLower, "is there a ") || strings.Contains(msgLower, "any mysql") ||
			strings.Contains(msgLower, "any redis") || strings.Contains(msgLower, "any mongo") ||
			strings.Contains(msgLower, "any postgres") || strings.Contains(msgLower, "any nginx") {
			cls.IsExistenceCheck = true
			cls.NeedsLiveData = true
			if cls.Intent == "" || cls.Intent == "knowledge" || cls.Intent == "chat" {
				cls.Intent = "list_pods"
			}
			if cls.FilterTerm == "" {
				cls.FilterTerm = extractAppFilter(msgLower)
			}
		}
	}

	if cls.Intent == "switch_namespace" && cls.Namespace != "" {
		session.Namespace = cls.Namespace
		resp := fmt.Sprintf("Switched to namespace **%s**.", cls.Namespace)
		e.addMessage(session, "assistant", resp, nil, "text")
		onChunk(StreamChunk{Type: "token", Content: resp})
		onChunk(StreamChunk{Type: "done"})
		return
	}

	if !cls.NeedsLiveData {
		if cls.DirectAnswer != "" {
			// Classifier provided the answer directly
			e.addMessage(session, "assistant", cls.DirectAnswer, nil, "text")
			onChunk(StreamChunk{Type: "token", Content: cls.DirectAnswer})
			onChunk(StreamChunk{Type: "done"})
			return
		}
		e.streamKnowledgeAnswer(ctx, session, userMsg, onChunk)
		return
	}

	// Pre-resolve pod name for describe/logs
	// Do this BEFORE Fetch so we can fill cls.ResourceName from session
	if cls.Intent == "describe_pod" || cls.Intent == "get_logs" {
		candidates := []string{}
		if cls.ResourceName != "" {
			candidates = append(candidates, cls.ResourceName)
		}
		if session.LastResource != "" {
			candidates = append(candidates, session.LastResource)
		}
		for _, candidate := range candidates {
			full, resolvedNS := e.k8s.ResolvePodName(ctx, "", candidate)
			if full != "" {
				cls.ResourceName = full
				cls.Namespace = resolvedNS
				e.mu.Lock()
				session.LastResource = full
				session.LastResourceNS = resolvedNS
				e.mu.Unlock()
				break
			}
		}
		// If still empty, show pod list immediately
		if cls.ResourceName == "" {
			tbl, _, _ := e.k8s.ListPods(ctx, "", "")
			if tbl != nil {
				onChunk(StreamChunk{Type: "data", Data: tbl, DataType: "table"})
			}
			onChunk(StreamChunk{Type: "token", Content: "Which pod would you like to inspect? Please specify from the list above."})
			onChunk(StreamChunk{Type: "done"})
			return
		}
	}

	// STAGE 2a: Fetch live data
	fd := e.Fetch(ctx, cls, session)

	// Update session context with resolved pod name
	if fd.PodName != "" {
		e.mu.Lock()
		session.LastResource = fd.PodName
		session.LastResourceNS = fd.PodNS
		e.mu.Unlock()
	} else if cls.ResourceName != "" {
		e.mu.Lock()
		session.LastResource = cls.ResourceName
		e.mu.Unlock()
	}

	// Handle existence checks in Go (never ask the model)
	if cls.IsExistenceCheck && cls.FilterTerm != "" {
		found, matched := fd.ExistenceCheck(cls.Intent, cls.FilterTerm)
		primaryTbl := fd.PrimaryTable(cls.Intent)
		if primaryTbl != nil {
			onChunk(StreamChunk{Type: "data", Data: primaryTbl, DataType: "table"})
		}
		var answer string
		if found {
			answer = fmt.Sprintf("✓ Yes — found **%d** result(s) matching \"%s\": %s",
				len(matched), cls.FilterTerm, strings.Join(matched, ", "))
		} else {
			totalRows := 0
			if primaryTbl != nil {
				totalRows = len(primaryTbl.Rows)
			}
			answer = fmt.Sprintf("✗ No — no resources matching \"%s\" found in the cluster (checked %d total resources).",
				cls.FilterTerm, totalRows)
		}
		e.addMessage(session, "assistant", answer, primaryTbl, "table")
		onChunk(StreamChunk{Type: "token", Content: answer})
		onChunk(StreamChunk{Type: "done"})
		return
	}

	if isListIntent(cls.Intent) {
		primaryTbl := fd.PrimaryTable(cls.Intent)
		if primaryTbl != nil {
			onChunk(StreamChunk{Type: "data", Data: primaryTbl, DataType: "table"})
		}
		answer := computeListSummary(cls.Intent, primaryTbl)
		e.addMessage(session, "assistant", answer, primaryTbl, "table")
		onChunk(StreamChunk{Type: "token", Content: answer})
		onChunk(StreamChunk{Type: "done"})
		return
	}

	if cls.Intent == "get_logs" {
		if errMsg, ok := fd.TextData["logs_error"]; ok {
			// No pod found - show pod list
			tbl, _, _ := e.k8s.ListPods(ctx, "", "")
			if tbl != nil {
				onChunk(StreamChunk{Type: "data", Data: tbl, DataType: "table"})
			}
			onChunk(StreamChunk{Type: "token", Content: errMsg + "\n\nPlease pick a pod from the list above."})
			onChunk(StreamChunk{Type: "done"})
			return
		}
		if fd.Logs != "" {
			onChunk(StreamChunk{Type: "data", Data: fd.Logs, DataType: "yaml"})
			// Stage 2: analyse the logs
			e.streamLogAnalysis(ctx, session, fd.PodName, fd.Logs, onChunk)
			return
		}
	}

	if cls.Intent == "describe_pod" {
		if fd.PodDescribe != "" {
			onChunk(StreamChunk{Type: "data", Data: fd.PodDescribe, DataType: "yaml"})
			podRef := fd.PodNS + "/" + fd.PodName
			prompt := fmt.Sprintf("Explain the configuration of pod **%s**. Summarise its status, resources, volumes, and any notable settings from the describe output above.", podRef)
			e.streamWithDataNS(ctx, session, prompt, fd.PodDescribe, "describe", fd.PodNS, onChunk)
			return
		} else {
			tbl, _, _ := e.k8s.ListPods(ctx, "", "")
			if tbl != nil {
				onChunk(StreamChunk{Type: "data", Data: tbl, DataType: "table"})
			}
			resName := cls.ResourceName
			if resName == "" { resName = session.LastResource }
			msg := "Which pod would you like to inspect? Pick from the list above."
			if resName != "" {
				msg = fmt.Sprintf("Could not find a pod matching **%s**. Here are all pods — pick the one you want:", resName)
			}
			onChunk(StreamChunk{Type: "token", Content: msg})
			onChunk(StreamChunk{Type: "done"})
			return
		}
	}

	switch cls.Intent {
	case "security_scan":
		result := e.runSecurityScan(ctx, "")
		onChunk(StreamChunk{Type: "data", Data: result, DataType: "yaml"})
		// Pass original question so AI knows to filter/focus appropriately
		e.streamWithData(ctx, session, userMsg, result, "diagnose", onChunk)
		return

	case "resource_limits":
		result := e.runResourceLimitsScan(ctx)
		onChunk(StreamChunk{Type: "data", Data: result, DataType: "yaml"})
		e.streamWithData(ctx, session, userMsg, result, "diagnose", onChunk)
		return

	case "analyze_all":
		rd := diagnostic.NewRuntimeDiagnostics(e.client)
		_, diagText, _ := rd.DiagnoseNamespace(ctx, "")
		pipelineText, _ := e.runFullAnalysis(ctx, "")
		// Auto-describe failing pods for richer context
		failingDescs := e.describeFailingPods(ctx, "")
		combined := diagText + "\n\n" + pipelineText + failingDescs
		e.streamWithData(ctx, session, userMsg, combined, "diagnose", onChunk)
		return
	}

	// STAGE 2b: Stream AI response with fetched data
	allText := fd.AllText()

	// Send primary table to UI
	if primaryTbl := fd.PrimaryTable(cls.Intent); primaryTbl != nil {
		onChunk(StreamChunk{Type: "data", Data: primaryTbl, DataType: "table"})
	}

	e.streamWithData(ctx, session, userMsg, allText, cls.AnswerStyle, onChunk)
}

// Stage 2 streaming helpers

// It sends Stage 2 LLM call with cluster data, namespace context, and streams response.
func (e *Engine) streamWithDataNS(ctx context.Context, session *Session, userMsg, clusterData, answerStyle, namespace string, onChunk func(StreamChunk)) {
	if namespace == "" {
		namespace = session.Namespace
	}
	systemPrompt := buildSystemPrompt(namespace, answerStyle)
	var contextBlock string
	if clusterData != "" {
		contextBlock = fmt.Sprintf("=== LIVE CLUSTER DATA (ground truth — fetched right now) ===\n%s\n=== END ===\n\n", clusterData)
	}

	prompt := contextBlock + userMsg

	messages := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}

	var full strings.Builder
	err := e.aiClient.GetChatCompletionStream(ctx, messages, func(token string) {
		full.WriteString(token)
		onChunk(StreamChunk{Type: "token", Content: token})
	})
	if err != nil {
		onChunk(StreamChunk{Type: "error", Content: fmt.Sprintf("AI error: %v", err)})
	}

	responseText := full.String()

	// Offer action buttons for suggested commands
	if cmds := ParseSuggestedCommands(responseText); len(cmds) > 0 {
		var actions []Action
		for _, cmd := range cmds {
			actions = append(actions, Action{
				Label:   "▶ Run: " + cmd,
				Command: "__exec__:" + cmd,
				Style:   "primary",
			})
		}
		actions = append(actions, Action{Label: "✕ Skip", Command: "__skip__", Style: "ghost"})
		onChunk(StreamChunk{Type: "confirm", Actions: actions})
	}

	e.addMessage(session, "assistant", responseText, nil, "text")
	onChunk(StreamChunk{Type: "done"})
}

func (e *Engine) streamWithData(ctx context.Context, session *Session, userMsg, clusterData, answerStyle string, onChunk func(StreamChunk)) {
	e.streamWithDataNS(ctx, session, userMsg, clusterData, answerStyle, session.Namespace, onChunk)
}

func (e *Engine) streamKnowledgeAnswer(ctx context.Context, session *Session, userMsg string, onChunk func(StreamChunk)) {
	summary, _ := e.k8s.GetClusterSummary(ctx)
	clusterCtx := ""
	if summary != "" {
		clusterCtx = "=== YOUR CLUSTER (for reference) ===\n" + summary + "\n"
	}
	e.streamWithData(ctx, session, userMsg, clusterCtx, "explain", onChunk)
}

func (e *Engine) streamLogAnalysis(ctx context.Context, session *Session, podName, logs string, onChunk func(StreamChunk)) {
	prompt := fmt.Sprintf(`Analyze these logs from pod **%s**.

IMPORTANT: Base your analysis ONLY on the actual log content below. Do NOT infer issues from the pod name.

Look for:
1. Lines starting with E (ERROR) or W (WARNING)
2. "error", "fatal", "panic", "failed", "timeout", "refused" keywords
3. Exit codes, OOMKilled, segfault mentions
4. Repeated failures suggesting a persistent issue

If the logs show no errors, say clearly: "Logs look healthy — no errors or warnings detected."

Logs:
%s`, podName, truncateLogs(logs, 4000))
	e.streamWithData(ctx, session, prompt, "", "logs", onChunk)
}




func (e *Engine) AnalyzeManifest(ctx context.Context, sessionID, content, filename string, onChunk func(StreamChunk)) {
	session := e.GetOrCreateSession(sessionID)
	e.addMessage(session, "user", "[Uploaded manifest: "+filename+"]", nil, "")

	reports := diagnostic.AnalyzeManifest(content, filename)
	if len(reports) == 0 {
		onChunk(StreamChunk{Type: "token", Content: "Could not parse the manifest. Please check it is valid YAML or JSON."})
		onChunk(StreamChunk{Type: "done"})
		return
	}

	var textSummary strings.Builder
	for _, r := range reports {
		textSummary.WriteString(diagnostic.FormatReport(r))
	}

	table := buildFindingTable(reports)
	if table != nil {
		onChunk(StreamChunk{Type: "data", Data: table, DataType: "table"})
	}

	prompt := buildManifestPrompt(content, textSummary.String())
	var full strings.Builder
	e.aiClient.GetChatCompletionStream(ctx, []ai.ChatMessage{
		{Role: "system", Content: manifestSystemPrompt},
		{Role: "user", Content: prompt},
	}, func(token string) {
		full.WriteString(token)
		onChunk(StreamChunk{Type: "token", Content: token})
	})
	e.addMessage(session, "assistant", full.String(), table, "table")
	onChunk(StreamChunk{Type: "done"})
}


func buildSystemPrompt(namespace, answerStyle string) string {
	nsHint := "all namespaces"
	if namespace != "" {
		nsHint = namespace
	}
	_ = nsHint
	styleInstructions := map[string]string{
		"diagnose": `Format each issue as:
**Problem**: <what is wrong>
Root Cause: <why it's happening>
Fix:
` + "```" + `
kubectl <exact command with real namespace, not placeholder>
` + "```" + `
IMPORTANT: Use REAL namespace names from the cluster data. NEVER use <namespace> or <pod-name> placeholders.
Current namespace context: ` + nsHint + `
Reference exact pod/service names from the cluster data.`,

		"describe": `Explain the resource configuration clearly. Highlight:
- Current status and health
- Resource limits and requests  
- Key configuration settings
- Any concerning values
Use the exact values from the cluster data.
NEVER use placeholder names like <namespace> or <pod-name> — use real names from the data.
Current namespace: ` + nsHint + ``,

		"logs": `Analyze the logs and identify:
1. Any ERROR or FATAL messages
2. Repeated warnings
3. Crash reasons (exit codes, OOM, etc.)
4. Unusual patterns
Be concise. Suggest a fix if the issue is clear.`,

		"explain": `You are an expert Kubernetes engineer and educator with deep knowledge of:
- Architecture: API server, etcd, scheduler, controller-manager, kubelet, kube-proxy
- Workloads: Pods, Deployments, StatefulSets, DaemonSets, Jobs, CronJobs, ReplicaSets
- Networking: Services (ClusterIP/NodePort/LoadBalancer), Ingress, NetworkPolicy, CNI, DNS
- Storage: PV, PVC, StorageClass, CSI drivers, volume types
- Security: RBAC, ServiceAccounts, PodSecurityStandards, Secrets, NetworkPolicy
- Operations: kubectl, Helm, resource limits/requests, HPA, VPA, namespaces
- Troubleshooting: OOMKilled, CrashLoopBackOff, ImagePullBackOff, Pending pods, evictions
- Observability: kubectl logs, events, describe, metrics-server, debugging

Answer guidelines:
1. Give clear, accurate explanations with real examples
2. Show exact kubectl commands with correct syntax
3. Include YAML snippets in code blocks when relevant
4. Mention common pitfalls and best practices
5. Reference live cluster data if provided and relevant

Use markdown headers for longer answers. Be practical and actionable.`,

		"yesno": `Answer YES or NO clearly, then list the relevant resources.`,

		"summarise": `Write 1-2 sentences summarising the data. Flag any issues.
Do NOT use Problem/Root Cause/Fix format for simple summaries.`,
	}

	style := styleInstructions[answerStyle]
	if style == "" {
		style = styleInstructions["explain"]
	}

	return fmt.Sprintf(`You are k8sdoc, an expert Kubernetes SRE assistant.
Current namespace: %s

CRITICAL RULES:
1. ONLY reference resource names that appear in the LIVE CLUSTER DATA block.
2. NEVER invent pod names, service names, or deployment names.
3. NEVER say "I don't have access to live data" — data is always provided above.
4. NEVER use placeholder names like <pod-name>, <namespace>, <container-name>.
5. If a resource is not in the data, say exactly which resources ARE available.
6. Use exact names from the data — copy them precisely.
7. For kubectl commands, use real names from the data, not invented examples.

RESPONSE STYLE for this query:
%s

Use markdown formatting. Keep answers focused and factual.`, namespace, style)
}



func isListIntent(intent string) bool {
	switch intent {
	case "list_pods", "list_deployments", "list_services", "list_nodes",
		"list_namespaces", "list_pvcs", "list_configmaps", "list_secrets",
		"list_ingress", "list_daemonsets", "list_statefulsets", "list_jobs",
		"list_events":
		return true
	}
	return false
}

func computeListSummary(intent string, tbl *TableData) string {
	if tbl == nil {
		return fmt.Sprintf("No %s found.", resourceKindFromAction(intent))
	}
	kind := resourceKindFromAction(intent)
	rowCount := len(tbl.Rows)
	if rowCount == 0 {
		return fmt.Sprintf("No %s found in the cluster.", kind)
	}
	pending, failed, highRestarts := countIssuesFromTable(tbl)
	if pending == 0 && failed == 0 && highRestarts == 0 {
		return fmt.Sprintf("%d %s, all healthy.", rowCount, kind)
	}
	parts := []string{fmt.Sprintf("%d %s total", rowCount, kind)}
	if pending > 0 {
		parts = append(parts, fmt.Sprintf("%d Pending", pending))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d Failed", failed))
	}
	if highRestarts > 0 {
		parts = append(parts, fmt.Sprintf("%d with high restarts", highRestarts))
	}
	return strings.Join(parts, " — ") + "."
}

func (e *Engine) addMessage(s *Session, role, content string, data any, dataType string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s.Messages = append(s.Messages, Message{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Role:      role,
		Content:   content,
		Data:      data,
		DataType:  dataType,
		Timestamp: time.Now(),
	})
	s.UpdatedAt = time.Now()
}

func (e *Engine) executeCommand(ctx context.Context, session *Session, cmd string, onChunk func(StreamChunk)) {
	onChunk(StreamChunk{Type: "token", Content: fmt.Sprintf("```\n$ %s\n```\n", cmd)})
	result := e.executor.Execute(ctx, cmd)
	if result.Error != "" {
		onChunk(StreamChunk{Type: "data", Data: fmt.Sprintf("Error: %s", result.Error), DataType: "yaml"})
		onChunk(StreamChunk{Type: "token", Content: fmt.Sprintf("\n⚠ Command failed: %s", result.Error)})
	} else {
		onChunk(StreamChunk{Type: "data", Data: result.Output, DataType: "yaml"})
		analysisPrompt := fmt.Sprintf("The user ran: `%s`\n\nOutput:\n%s\n\nBriefly summarise what this output shows. Flag any issues. 2-3 sentences max.",
			cmd, truncateLogs(result.Output, 3000))
		e.streamWithData(ctx, session, analysisPrompt, "", "explain", onChunk)
		return
	}
	onChunk(StreamChunk{Type: "done"})
}


func (e *Engine) runSecurityScan(ctx context.Context, _ string) string {
	var sb strings.Builder
	sb.WriteString("=== SECURITY SCAN RESULTS ===\n")
	pods, err := e.client.GetClient().CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "Error: " + err.Error()
	}
	critical, high, medium, low := 0, 0, 0, 0
	for _, pod := range pods.Items {
		podRef := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
		if pod.Spec.HostNetwork {
			sb.WriteString(fmt.Sprintf("[CRITICAL] %s — hostNetwork: true\n", podRef))
			critical++
		}
		if pod.Spec.HostPID {
			sb.WriteString(fmt.Sprintf("[CRITICAL] %s — hostPID: true\n", podRef))
			critical++
		}
		for _, cont := range pod.Spec.Containers {
			cRef := fmt.Sprintf("%s container=%s", podRef, cont.Name)
			sc := cont.SecurityContext
			runAsRoot := sc == nil || sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot
			runAsUser := ""
			if sc != nil && sc.RunAsUser != nil {
				runAsUser = fmt.Sprintf(" (runAsUser=%d)", *sc.RunAsUser)
				if *sc.RunAsUser == 0 {
					runAsRoot = true
				}
			}
			if runAsRoot {
				sb.WriteString(fmt.Sprintf("[HIGH]     %s — may run as root%s\n", cRef, runAsUser))
				high++
			}
			if sc != nil && sc.Privileged != nil && *sc.Privileged {
				sb.WriteString(fmt.Sprintf("[CRITICAL] %s — privileged: true\n", cRef))
				critical++
			}
			if sc == nil || sc.AllowPrivilegeEscalation == nil {
				sb.WriteString(fmt.Sprintf("[MEDIUM]   %s — allowPrivilegeEscalation not set to false\n", cRef))
				medium++
			} else if *sc.AllowPrivilegeEscalation {
				sb.WriteString(fmt.Sprintf("[HIGH]     %s — allowPrivilegeEscalation: true\n", cRef))
				high++
			}
			if sc == nil || sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
				sb.WriteString(fmt.Sprintf("[LOW]      %s — readOnlyRootFilesystem not set\n", cRef))
				low++
			}
			if cont.Resources.Limits == nil {
				sb.WriteString(fmt.Sprintf("[MEDIUM]   %s — no resource limits\n", cRef))
				medium++
			}
			if !strings.Contains(cont.Image, ":") || strings.HasSuffix(cont.Image, ":latest") {
				sb.WriteString(fmt.Sprintf("[LOW]      %s — image %q untagged/latest\n", cRef, cont.Image))
				low++
			}
		}
	}
	sb.WriteString(fmt.Sprintf("\nSUMMARY: %d Critical, %d High, %d Medium, %d Low\n", critical, high, medium, low))
	return sb.String()
}

func (e *Engine) runResourceLimitsScan(ctx context.Context) string {
	var sb strings.Builder
	sb.WriteString("=== RESOURCE LIMITS SCAN ===\n")
	sb.WriteString(fmt.Sprintf("%-50s %-12s %-12s %-12s %-12s\n",
		"POD/CONTAINER", "CPU-REQ", "CPU-LIM", "MEM-REQ", "MEM-LIM"))
	sb.WriteString(strings.Repeat("-", 100) + "\n")
	pods, err := e.client.GetClient().CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "Error: " + err.Error()
	}
	noLimits := 0
	for _, pod := range pods.Items {
		for _, cont := range pod.Spec.Containers {
			cpuReq := cont.Resources.Requests.Cpu().String()
			cpuLim := cont.Resources.Limits.Cpu().String()
			memReq := cont.Resources.Requests.Memory().String()
			memLim := cont.Resources.Limits.Memory().String()
			if cpuLim == "0" || memLim == "0" || cpuReq == "0" || memReq == "0" {
				noLimits++
				ref := fmt.Sprintf("%s/%s:%s", pod.Namespace, pod.Name, cont.Name)
				if len(ref) > 49 {
					ref = ref[:46] + "..."
				}
				f := func(s string) string {
					if s == "0" {
						return "NOT SET"
					}
					return s
				}
				sb.WriteString(fmt.Sprintf("%-50s %-12s %-12s %-12s %-12s\n",
					ref, f(cpuReq), f(cpuLim), f(memReq), f(memLim)))
			}
		}
	}
	sb.WriteString(fmt.Sprintf("\n%d container(s) missing limits/requests.\n", noLimits))
	return sb.String()
}

func (e *Engine) runFullAnalysis(ctx context.Context, namespace string) (string, error) {
	cfg, err := analysis.NewAnalysisWithClient(e.client, e.aiClient, namespace)
	if err != nil {
		return "", err
	}
	cfg.CollectProbeResults()
	cfg.RunAnalysis()
	out, err := cfg.PrintOutput("text")
	if err != nil {
		return "", err
	}
	return string(out), nil
}


const manifestSystemPrompt = `You are a Kubernetes manifest expert. Analyze the YAML and the findings.
For each issue: explain WHY it is a problem, show the exact YAML fix.
Prioritize errors over warnings. Use code blocks for YAML.`

func buildManifestPrompt(content, findings string) string {
	if len(content) > 3000 {
		content = content[:3000] + "\n...(truncated)"
	}
	return fmt.Sprintf("=== MANIFEST ===\n%s\n\n=== FINDINGS ===\n%s\n\nExplain issues and provide exact fixes.", content, findings)
}

func buildFindingTable(reports []diagnostic.ManifestReport) *TableData {
	if len(reports) == 0 {
		return nil
	}
	table := &TableData{
		Title:   "Manifest Analysis",
		Headers: []string{"SEVERITY", "RESOURCE", "RULE", "MESSAGE"},
	}
	for _, r := range reports {
		rname := r.Kind + "/" + r.Name
		if len(r.Findings) == 0 {
			table.Rows = append(table.Rows, TableRow{
				"SEVERITY": "ok", "RESOURCE": rname,
				"RULE": "clean", "MESSAGE": fmt.Sprintf("No issues (score: %d/100)", r.Score),
			})
			continue
		}
		for _, f := range r.Findings {
			table.Rows = append(table.Rows, TableRow{
				"SEVERITY": string(f.Severity), "RESOURCE": rname,
				"RULE": f.Rule, "MESSAGE": f.Message,
			})
		}
	}
	return table
}


func truncateLogs(logs string, maxLen int) string {
	if len(logs) <= maxLen {
		return logs
	}
	return "...(truncated)\n" + logs[len(logs)-maxLen:]
}

func resourceKindFromAction(action string) string {
	m := map[string]string{
		"list_pods": "pods", "list_deployments": "deployments",
		"list_services": "services", "list_nodes": "nodes",
		"list_namespaces": "namespaces", "list_pvcs": "PersistentVolumeClaims",
		"list_configmaps": "ConfigMaps", "list_secrets": "Secrets",
		"list_ingress": "Ingresses", "list_daemonsets": "DaemonSets",
		"list_statefulsets": "StatefulSets", "list_jobs": "Jobs",
		"list_events": "Events",
	}
	if v, ok := m[action]; ok {
		return v
	}
	return "results"
}

func countIssuesFromTable(table *TableData) (pending, failed, highRestarts int) {
	for _, row := range table.Rows {
		if status, ok := row["STATUS"]; ok {
			sl := strings.ToLower(status)
			if sl == "pending" {
				pending++
			} else if sl == "failed" || sl == "error" {
				failed++
			}
		}
		if restarts, ok := row["RESTARTS"]; ok {
			var n int
			fmt.Sscanf(restarts, "%d", &n)
			if n > 5 {
				highRestarts++
			}
		}
	}
	return
}

func extractAppFilter(lower string) string {
	skipWords := map[string]bool{
		"pod": true, "pods": true, "deployment": true, "deployments": true,
		"service": true, "services": true, "container": true, "containers": true,
		"node": true, "nodes": true, "namespace": true, "namespaces": true,
		"secret": true, "secrets": true, "volume": true, "volumes": true,
		"pvc": true, "running": true, "error": true, "errors": true,
		"any": true, "the": true, "a": true, "an": true, "there": true,
		"is": true, "are": true, "check": true, "if": true, "or": true,
		"not": true, "still": true, "related": true, "to": true, "in": true,
	}
	words := strings.Fields(lower)
	for _, w := range words {
		w = strings.Trim(w, ".,;?!'\"")
		if !skipWords[w] && len(w) > 2 {
			return w
		}
	}
	return ""
}

func isValidNamespaceName(s string) bool {
	if s == "" || len(s) > 63 || strings.Contains(s, " ") {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	nonNS := map[string]bool{
		"status": true, "check": true, "connectivity": true, "dns": true,
		"health": true, "running": true, "failing": true, "pending": true,
		"error": true, "cluster": true, "pod": true, "node": true,
	}
	return !nonNS[s]
}


func (e *Engine) describeFailingPods(ctx context.Context, namespace string) string {
	pods, err := e.client.GetClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return ""
	}
	var sb strings.Builder
	badStates := map[string]bool{
		"ImagePullBackOff": true, "ErrImagePull": true,
		"CrashLoopBackOff": true, "Error": true,
		"OOMKilled": true, "Pending": true, "Failed": true,
	}
	count := 0
	for _, pod := range pods.Items {
		if count >= 5 { break } // limit to 5 to avoid huge context
		isBad := false
		if badStates[string(pod.Status.Phase)] {
			isBad = true
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil && badStates[cs.State.Waiting.Reason] {
				isBad = true
			}
			if cs.State.Terminated != nil && cs.State.Terminated.Reason == "OOMKilled" {
				isBad = true
			}
			if cs.RestartCount > 5 {
				isBad = true
			}
		}
		if !isBad { continue }
		desc, err := e.k8s.DescribePod(ctx, pod.Namespace, pod.Name)
		if err == nil {
			sb.WriteString(fmt.Sprintf("\n=== DESCRIBE: %s/%s ===\n%s\n", pod.Namespace, pod.Name, desc))
			count++
		}
	}
	return sb.String()
}
