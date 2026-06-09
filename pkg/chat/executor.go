package chat

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/user/k8sdoc/pkg/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


var SafeCommands = map[string]bool{
	"get":      true,
	"describe": true,
	"logs":     true,
	"top":      true,
	"explain":  true,
	"diff":     true,
	"version":  true,
	"cluster-info": true,
}

type CommandResult struct {
	Command string
	Output  string
	Error   string
	Runtime time.Duration
}

type Executor struct {
	client *kubernetes.Client
}

func NewExecutor(client *kubernetes.Client) *Executor {
	return &Executor{client: client}
}

func (ex *Executor) Execute(ctx context.Context, command string) CommandResult {
	start := time.Now()
	command = strings.TrimSpace(command)

	result := CommandResult{Command: command}

	if pipeIdx := strings.Index(command, "|"); pipeIdx != -1 {
		kubectlPart := strings.TrimSpace(command[:pipeIdx])
		result.Command = kubectlPart + " (pipe removed — showing full output)"
		command = kubectlPart
	}

	parts := parseCommand(command)
	if len(parts) == 0 {
		result.Error = "empty command"
		return result
	}

	if parts[0] == "kubectl" {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		result.Error = "no kubectl verb specified"
		return result
	}

	verb := parts[0]

	if !SafeCommands[verb] {
		result.Error = fmt.Sprintf("verb %q is not in the safe command list. k8sdoc only executes read-only commands (get, describe, logs, top, explain).", verb)
		return result
	}

	// Route to the appropriate handler
	var err error
	switch verb {
	case "get":
		result.Output, err = ex.runGet(ctx, parts[1:])
	case "describe":
		result.Output, err = ex.runDescribe(ctx, parts[1:])
	case "logs":
		result.Output, err = ex.runLogs(ctx, parts[1:])
	case "top":
		result.Output, err = ex.runTop(ctx, parts[1:])
	default:
		result.Output, err = ex.runKubectl(ctx, parts)
	}

	if err != nil {
		result.Error = err.Error()
	}
	result.Runtime = time.Since(start)
	return result
}

func (ex *Executor) runGet(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: kubectl get <resource>")
	}

	for _, a := range args {
		if strings.HasPrefix(a, "-o") || a == "jsonpath" || strings.Contains(a, "jsonpath") {
			return ex.runKubectl(ctx, append([]string{"get"}, args...))
		}
	}

	resource := args[0]
	namespace := ""
	name := ""
	allNS := false

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-n", "--namespace":
			if i+1 < len(args) {
				namespace = args[i+1]
				i++
			}
		case "-A", "--all-namespaces":
			allNS = true
		case "-o", "--output":
			return ex.runKubectl(ctx, append([]string{"get"}, args...))
		default:
			if !strings.HasPrefix(args[i], "-") && name == "" {
				name = args[i]
			}
		}
	}
	if allNS {
		namespace = ""
	}

	tools := NewK8sTools(ex.client)

	switch strings.ToLower(resource) {
	case "pods", "pod", "po":
		_, text, err := tools.ListPods(ctx, namespace, "")
		return text, err
	case "deployments", "deployment", "deploy":
		_, text, err := tools.ListDeployments(ctx, namespace)
		return text, err
	case "services", "service", "svc":
		_, text, err := tools.ListServices(ctx, namespace)
		return text, err
	case "nodes", "node", "no":
		_, text, err := tools.ListNodes(ctx)
		return text, err
	case "namespaces", "namespace", "ns":
		_, text, err := tools.ListNamespaces(ctx)
		return text, err
	case "events", "event", "ev":
		_, text, err := tools.ListEvents(ctx, namespace, false)
		return text, err
	case "secrets", "secret":
		_, text, err := tools.ListSecrets(ctx, namespace)
		return text, err
	case "configmaps", "configmap", "cm":
		_, text, err := tools.ListConfigMaps(ctx, namespace)
		return text, err
	case "pvc", "persistentvolumeclaims":
		_, text, err := tools.ListPVCs(ctx, namespace)
		return text, err
	case "ingresses", "ingress", "ing":
		_, text, err := tools.ListIngresses(ctx, namespace)
		return text, err
	case "daemonsets", "daemonset", "ds":
		_, text, err := tools.ListDaemonSets(ctx, namespace)
		return text, err
	case "statefulsets", "statefulset", "sts":
		_, text, err := tools.ListStatefulSets(ctx, namespace)
		return text, err
	case "jobs", "job":
		_, text, err := tools.ListJobs(ctx, namespace)
		return text, err
	default:
		// Try kubectl binary as fallback
		return ex.runKubectl(ctx, append([]string{"get"}, args...))
	}

}

func (ex *Executor) runDescribe(ctx context.Context, args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: kubectl describe <resource> <name>")
	}
	resource := args[0]
	name := args[1]
	namespace := ""
	for i := 2; i < len(args); i++ {
		if (args[i] == "-n" || args[i] == "--namespace") && i+1 < len(args) {
			namespace = args[i+1]
			i++
		}
	}

	tools := NewK8sTools(ex.client)

	switch strings.ToLower(resource) {
	case "pod", "pods", "po":
		fullName, resolvedNS := tools.ResolvePodName(ctx, namespace, name)
		if fullName != "" {
			name = fullName
			namespace = resolvedNS
		} else if namespace == "" {
			namespace = "default"
		}
		return tools.DescribePod(ctx, namespace, name)
	case "node", "nodes", "no":
		node, err := ex.client.GetClient().CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Node: %s\n", node.Name))
		status := "Unknown"
		for _, cond := range node.Status.Conditions {
			if string(cond.Type) == "Ready" {
				if cond.Status == "True" { status = "Ready" } else { status = "NotReady" }
			}
		}
		sb.WriteString(fmt.Sprintf("Status: %s\n", status))
		sb.WriteString(fmt.Sprintf("CPU: %s\n", node.Status.Capacity.Cpu().String()))
		sb.WriteString(fmt.Sprintf("Memory: %s\n", node.Status.Capacity.Memory().String()))
		sb.WriteString(fmt.Sprintf("Kubelet: %s\n", node.Status.NodeInfo.KubeletVersion))
		for _, cond := range node.Status.Conditions {
			sb.WriteString(fmt.Sprintf("Condition %s: %s\n", cond.Type, cond.Status))
		}
		return sb.String(), nil
	default:
		return ex.runKubectl(ctx, append([]string{"describe"}, args...))
	}
}

func (ex *Executor) runLogs(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: kubectl logs <pod-name>")
	}
	podName := args[0]
	namespace := ""
	container := ""
	var lines int64 = 100

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-n", "--namespace":
			if i+1 < len(args) {
				namespace = args[i+1]
				i++
			}
		case "-c", "--container":
			if i+1 < len(args) {
				container = args[i+1]
				i++
			}
		case "--tail":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &lines)
				i++
			}
		}
	}

	tools := NewK8sTools(ex.client)
	fullName, resolvedNS := tools.ResolvePodName(ctx, namespace, podName)
	if fullName != "" {
		podName = fullName
		namespace = resolvedNS
	} else if namespace == "" {
		namespace = "default"
	}

	return tools.GetPodLogs(ctx, namespace, podName, container, lines)
}

func (ex *Executor) runTop(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: kubectl top pods|nodes")
	}
	return ex.runKubectl(ctx, append([]string{"top"}, args...))
}


func (ex *Executor) runKubectl(ctx context.Context, args []string) (string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "kubectl", args...)
	out, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	if err != nil {
		if outStr != "" {
			return outStr, nil
		}
		return "", fmt.Errorf("kubectl %s: %w", strings.Join(args, " "), err)
	}
	if outStr == "" {
		return "(no output — result may be empty)", nil
	}
	return outStr, nil
}


func ParseSuggestedCommands(text string) []string {
	var commands []string
	seen := map[string]bool{}

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "kubectl ") {
			cmd := extractKubectlCommand(line)
			if cmd != "" && !seen[cmd] && isSafeKubectlCommand(cmd) {
				commands = append(commands, cmd)
				seen[cmd] = true
			}
		}
	}
	return commands
}

func extractKubectlCommand(line string) string {
	line = strings.TrimPrefix(line, "```")
	line = strings.TrimSuffix(line, "```")
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "kubectl ") {
		return line
	}
	return ""
}

func isSafeKubectlCommand(cmd string) bool {
	parts := parseCommand(cmd)
	if len(parts) < 2 {
		return false
	}
	return SafeCommands[parts[1]]
}

func parseCommand(cmd string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, ch := range cmd {
		switch {
		case inQuote && ch == quoteChar:
			inQuote = false
		case !inQuote && (ch == '\'' || ch == '"'):
			inQuote = true
			quoteChar = ch
		case !inQuote && ch == ' ':
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}


