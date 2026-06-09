package diagnostic

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

type Finding struct {
	Severity Severity `json:"severity"`
	Rule     string   `json:"rule"`
	Message  string   `json:"message"`
	Field    string   `json:"field,omitempty"`
	Fix      string   `json:"fix,omitempty"`
	Line     int      `json:"line,omitempty"`
}

type ManifestReport struct {
	Filename string    `json:"filename"`
	Kind     string    `json:"kind"`
	Name     string    `json:"name"`
	Findings []Finding `json:"findings"`
	Score    int       `json:"score"` // 0-100, 100 = perfect
}

func AnalyzeManifest(content, filename string) []ManifestReport {
	var reports []ManifestReport

	docs := splitYAMLDocs(content)
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" || doc == "---" {
			continue
		}
		report := analyzeDoc(doc, filename)
		reports = append(reports, report)
	}
	return reports
}

func analyzeDoc(content, filename string) ManifestReport {
	report := ManifestReport{Filename: filename}

	var obj map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &obj); err != nil {
		report.Findings = append(report.Findings, Finding{
			Severity: SeverityError,
			Rule:     "yaml-parse-error",
			Message:  fmt.Sprintf("Invalid YAML: %v", err),
			Fix:      "Fix YAML syntax — check indentation, colons, and quotes",
		})
		report.Score = 0
		return report
	}
	if obj == nil {
		return report
	}

	report.Kind = strVal(obj, "kind")
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		report.Name = strVal(meta, "name")
	}

	switch strings.ToLower(report.Kind) {
	case "deployment", "statefulset", "daemonset", "replicaset":
		report.Findings = append(report.Findings, checkWorkload(obj, report.Kind)...)
	case "pod":
		spec := getMap(obj, "spec")
		report.Findings = append(report.Findings, checkPodSpec(spec, "spec")...)
	case "service":
		report.Findings = append(report.Findings, checkService(obj)...)
	case "ingress":
		report.Findings = append(report.Findings, checkIngress(obj)...)
	case "configmap":
		report.Findings = append(report.Findings, checkConfigMap(obj)...)
	case "secret":
		report.Findings = append(report.Findings, checkSecret(obj)...)
	case "persistentvolumeclaim":
		report.Findings = append(report.Findings, checkPVC(obj)...)
	case "horizontalpodautoscaler":
		report.Findings = append(report.Findings, checkHPA(obj)...)
	case "networkpolicy":
		report.Findings = append(report.Findings, checkNetworkPolicy(obj)...)
	case "serviceaccount":
	default:
		if report.Kind == "" {
			report.Findings = append(report.Findings, Finding{
				Severity: SeverityError,
				Rule:     "missing-kind",
				Message:  "Missing required field: kind",
				Fix:      "Add 'kind: Deployment' (or appropriate kind)",
			})
		}
	}

	report.Findings = append(report.Findings, checkUniversal(obj)...)

	// Calculate score
	report.Score = calcScore(report.Findings)
	return report
}

func checkUniversal(obj map[string]interface{}) []Finding {
	var findings []Finding

	if strVal(obj, "apiVersion") == "" {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Rule:     "missing-api-version",
			Message:  "Missing required field: apiVersion",
			Fix:      "Add 'apiVersion: apps/v1' or appropriate version",
		})
	}

	meta := getMap(obj, "metadata")
	if strVal(meta, "name") == "" {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Rule:     "missing-name",
			Message:  "Missing required field: metadata.name",
			Fix:      "Add 'name: my-resource' under metadata",
		})
	}

	labels := getMap(meta, "labels")
	if len(labels) == 0 {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Rule:     "missing-labels",
			Message:  "No labels defined on resource — labels are important for selection and filtering",
			Fix:      "Add labels under metadata.labels (e.g. app: myapp, version: v1)",
		})
	}

	return findings
}

func checkWorkload(obj map[string]interface{}, kind string) []Finding {
	var findings []Finding

	spec := getMap(obj, "spec")
	template := getMap(spec, "template")
	templateSpec := getMap(template, "spec")
	selector := getMap(spec, "selector")
	matchLabels := getMap(selector, "matchLabels")
	templateMeta := getMap(template, "metadata")
	templateLabels := getMap(templateMeta, "labels")

	if len(matchLabels) == 0 {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Rule:     "missing-selector",
			Message:  "spec.selector.matchLabels is empty — pods will never be selected",
			Field:    "spec.selector.matchLabels",
			Fix:      "Add matchLabels that match your pod template labels",
		})
	}

	for k, v := range matchLabels {
		if tv, ok := templateLabels[k]; !ok || tv != v {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Rule:     "selector-label-mismatch",
				Message:  fmt.Sprintf("Selector label %s=%v not found in pod template labels — deployment will have no pods", k, v),
				Field:    "spec.template.metadata.labels",
				Fix:      fmt.Sprintf("Add '%s: %v' to spec.template.metadata.labels", k, v),
			})
		}
	}

	if kind == "deployment" || kind == "statefulset" {
		if replicas, ok := spec["replicas"]; ok {
			r := toInt(replicas)
			if r == 0 {
				findings = append(findings, Finding{
					Severity: SeverityWarning,
					Rule:     "zero-replicas",
					Message:  "spec.replicas is 0 — no pods will be created",
					Field:    "spec.replicas",
					Fix:      "Set replicas to at least 1",
				})
			}
			if r == 1 && kind == "deployment" {
				findings = append(findings, Finding{
					Severity: SeverityInfo,
					Rule:     "single-replica",
					Message:  "Only 1 replica — no high availability. Consider replicas: 2 or more for production",
					Field:    "spec.replicas",
					Fix:      "Set replicas: 2 or more for production workloads",
				})
			}
		}
	}

	findings = append(findings, checkPodSpec(templateSpec, "spec.template.spec")...)

	if kind == "deployment" {
		strategy := getMap(spec, "strategy")
		if strVal(strategy, "type") == "" {
			findings = append(findings, Finding{
				Severity: SeverityInfo,
				Rule:     "no-update-strategy",
				Message:  "No update strategy defined — defaults to RollingUpdate",
				Fix:      "Explicitly set spec.strategy.type: RollingUpdate for clarity",
			})
		}
	}

	return findings
}


func checkPodSpec(spec map[string]interface{}, path string) []Finding {
	var findings []Finding

	containers, _ := spec["containers"].([]interface{})
	if len(containers) == 0 {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Rule:     "no-containers",
			Message:  "No containers defined in pod spec",
			Fix:      "Add at least one container under spec.containers",
		})
		return findings
	}

	for _, c := range containers {
		container, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		findings = append(findings, checkContainer(container, path+".containers")...)
	}

	initContainers, _ := spec["initContainers"].([]interface{})
	for _, c := range initContainers {
		container, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		findings = append(findings, checkContainer(container, path+".initContainers")...)
	}

	if hostNetwork, ok := spec["hostNetwork"].(bool); ok && hostNetwork {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Rule:     "host-network",
			Message:  "hostNetwork: true — pod shares the node's network namespace, security risk",
			Field:    path + ".hostNetwork",
			Fix:      "Only use hostNetwork if absolutely required",
		})
	}

	if strVal(spec, "serviceAccountName") == "default" || strVal(spec, "serviceAccountName") == "" {
		findings = append(findings, Finding{
			Severity: SeverityInfo,
			Rule:     "default-service-account",
			Message:  "Using default service account — create a dedicated service account with minimal permissions",
			Fix:      "Create a ServiceAccount and set spec.serviceAccountName",
		})
	}

	return findings
}

func checkContainer(c map[string]interface{}, path string) []Finding {
	var findings []Finding
	name := strVal(c, "name")
	cpath := path + "[" + name + "]"

	image := strVal(c, "image")
	if image == "" {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Rule:     "missing-image",
			Message:  fmt.Sprintf("Container %q has no image", name),
			Field:    cpath + ".image",
			Fix:      "Set image: nginx:1.25 (use explicit version tags)",
		})
	} else {
		if strings.HasSuffix(image, ":latest") || !strings.Contains(image, ":") {
			findings = append(findings, Finding{
				Severity: SeverityWarning,
				Rule:     "image-latest-tag",
				Message:  fmt.Sprintf("Container %q uses 'latest' or untagged image %q — not reproducible", name, image),
				Field:    cpath + ".image",
				Fix:      "Pin to a specific version tag: nginx:1.25.3",
			})
		}
	}

	resources := getMap(c, "resources")
	requests := getMap(resources, "requests")
	limits := getMap(resources, "limits")

	if len(requests) == 0 && len(limits) == 0 {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Rule:     "no-resource-limits",
			Message:  fmt.Sprintf("Container %q has no resource requests or limits — can starve other pods", name),
			Field:    cpath + ".resources",
			Fix:      "Add resources.requests and resources.limits for cpu and memory",
		})
	} else {
		if _, ok := requests["cpu"]; !ok {
			findings = append(findings, Finding{
				Severity: SeverityWarning,
				Rule:     "missing-cpu-request",
				Message:  fmt.Sprintf("Container %q missing resources.requests.cpu", name),
				Field:    cpath + ".resources.requests.cpu",
				Fix:      "Add: resources:\n  requests:\n    cpu: 100m",
			})
		}
		if _, ok := requests["memory"]; !ok {
			findings = append(findings, Finding{
				Severity: SeverityWarning,
				Rule:     "missing-memory-request",
				Message:  fmt.Sprintf("Container %q missing resources.requests.memory", name),
				Field:    cpath + ".resources.requests.memory",
				Fix:      "Add: resources:\n  requests:\n    memory: 128Mi",
			})
		}
		if _, ok := limits["memory"]; !ok {
			findings = append(findings, Finding{
				Severity: SeverityWarning,
				Rule:     "missing-memory-limit",
				Message:  fmt.Sprintf("Container %q missing resources.limits.memory — OOM risk", name),
				Field:    cpath + ".resources.limits.memory",
				Fix:      "Add: resources:\n  limits:\n    memory: 256Mi",
			})
		}


		reqMem := parseMemory(strValI(requests, "memory"))
		limMem := parseMemory(strValI(limits, "memory"))
		if reqMem > 0 && limMem > 0 && reqMem > limMem {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Rule:     "request-exceeds-limit",
				Message:  fmt.Sprintf("Container %q memory request (%s) > limit (%s) — pod will never schedule", name, strValI(requests, "memory"), strValI(limits, "memory")),
				Field:    cpath + ".resources",
				Fix:      "Set requests.memory <= limits.memory",
			})
		}
	}

	if _, ok := c["livenessProbe"]; !ok {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Rule:     "missing-liveness-probe",
			Message:  fmt.Sprintf("Container %q has no livenessProbe — Kubernetes can't detect if it hangs", name),
			Field:    cpath + ".livenessProbe",
			Fix:      "Add livenessProbe with httpGet, exec, or tcpSocket",
		})
	}

	if _, ok := c["readinessProbe"]; !ok {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Rule:     "missing-readiness-probe",
			Message:  fmt.Sprintf("Container %q has no readinessProbe — traffic sent before app is ready", name),
			Field:    cpath + ".readinessProbe",
			Fix:      "Add readinessProbe to prevent premature traffic routing",
		})
	}

	secCtx := getMap(c, "securityContext")
	if len(secCtx) == 0 {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Rule:     "missing-security-context",
			Message:  fmt.Sprintf("Container %q has no securityContext — runs as root by default", name),
			Field:    cpath + ".securityContext",
			Fix:      "Add: securityContext:\n  runAsNonRoot: true\n  readOnlyRootFilesystem: true\n  allowPrivilegeEscalation: false",
		})
	} else {
		if priv, ok := secCtx["privileged"].(bool); ok && priv {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Rule:     "privileged-container",
				Message:  fmt.Sprintf("Container %q runs as privileged — full host access, severe security risk", name),
				Field:    cpath + ".securityContext.privileged",
				Fix:      "Remove privileged: true unless absolutely required",
			})
		}
		if ape, ok := secCtx["allowPrivilegeEscalation"]; ok {
			if apeB, ok := ape.(bool); ok && apeB {
				findings = append(findings, Finding{
					Severity: SeverityWarning,
					Rule:     "privilege-escalation-allowed",
					Message:  fmt.Sprintf("Container %q allows privilege escalation", name),
					Field:    cpath + ".securityContext.allowPrivilegeEscalation",
					Fix:      "Set allowPrivilegeEscalation: false",
				})
			}
		}
	}

	envVars, _ := c["env"].([]interface{})
	sensitiveKeys := []string{"password", "secret", "token", "key", "api_key", "passwd", "credential"}
	for _, e := range envVars {
		env, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		envName := strings.ToLower(strVal(env, "name"))
		_, hasValue := env["value"]
		for _, sk := range sensitiveKeys {
			if strings.Contains(envName, sk) && hasValue {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Rule:     "secret-in-plain-env",
					Message:  fmt.Sprintf("Container %q has sensitive env var %q as plain text — use a Secret", name, strVal(env, "name")),
					Field:    cpath + ".env",
					Fix:      "Use valueFrom.secretKeyRef instead of plain value for sensitive variables",
				})
			}
		}
	}

	pullPolicy := strVal(c, "imagePullPolicy")
	if pullPolicy == "Always" && !strings.HasSuffix(strVal(c, "image"), ":latest") {
		findings = append(findings, Finding{
			Severity: SeverityInfo,
			Rule:     "unnecessary-always-pull",
			Message:  fmt.Sprintf("Container %q uses imagePullPolicy: Always with pinned tag — wastes bandwidth", name),
			Fix:      "Use imagePullPolicy: IfNotPresent for pinned image tags",
		})
	}

	return findings
}


func checkService(obj map[string]interface{}) []Finding {
	var findings []Finding
	spec := getMap(obj, "spec")

	svcType := strVal(spec, "type")
	if svcType == "NodePort" {
		findings = append(findings, Finding{
			Severity: SeverityInfo,
			Rule:     "nodeport-service",
			Message:  "Service type NodePort exposes a port on every node — use LoadBalancer or Ingress for production",
		})
	}

	selector := getMap(spec, "selector")
	if len(selector) == 0 {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Rule:     "empty-service-selector",
			Message:  "Service has no selector — will not route to any pods (headless or external service?)",
			Field:    "spec.selector",
			Fix:      "Add selector labels matching your pod template labels",
		})
	}

	ports, _ := spec["ports"].([]interface{})
	if len(ports) == 0 {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Rule:     "no-service-ports",
			Message:  "Service has no ports defined",
			Fix:      "Add ports under spec.ports",
		})
	}

	return findings
}


func checkIngress(obj map[string]interface{}) []Finding {
	var findings []Finding
	spec := getMap(obj, "spec")

	// TLS check
	tls, _ := spec["tls"].([]interface{})
	rules, _ := spec["rules"].([]interface{})
	if len(tls) == 0 && len(rules) > 0 {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Rule:     "ingress-no-tls",
			Message:  "Ingress has no TLS configuration — traffic is unencrypted",
			Fix:      "Add spec.tls with a certificate secret, or use cert-manager",
		})
	}

	if strVal(spec, "ingressClassName") == "" {
		meta := getMap(obj, "metadata")
		annotations := getMap(meta, "annotations")
		if _, ok := annotations["kubernetes.io/ingress.class"]; !ok {
			findings = append(findings, Finding{
				Severity: SeverityWarning,
				Rule:     "ingress-no-class",
				Message:  "No ingressClassName or kubernetes.io/ingress.class annotation — may not be picked up by any controller",
				Fix:      "Add spec.ingressClassName: nginx (or your ingress controller name)",
			})
		}
	}

	return findings
}


func checkConfigMap(obj map[string]interface{}) []Finding {
	var findings []Finding
	data, _ := obj["data"].(map[string]interface{})
	sensitiveKeys := []string{"password", "secret", "token", "api_key", "private_key", "passwd"}
	for k := range data {
		kLower := strings.ToLower(k)
		for _, sk := range sensitiveKeys {
			if strings.Contains(kLower, sk) {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Rule:     "secret-in-configmap",
					Message:  fmt.Sprintf("ConfigMap key %q looks like a secret — use a Secret resource instead", k),
					Fix:      "Move sensitive values to a Secret and reference with secretKeyRef",
				})
			}
		}
	}
	return findings
}


func checkSecret(obj map[string]interface{}) []Finding {
	var findings []Finding
	secretType := strVal(obj, "type")
	if secretType == "" {
		findings = append(findings, Finding{
			Severity: SeverityInfo,
			Rule:     "secret-no-type",
			Message:  "Secret has no type — defaults to Opaque",
			Fix:      "Explicitly set type: Opaque for clarity",
		})
	}

	data, _ := obj["data"].(map[string]interface{})
	for k, v := range data {
		val := fmt.Sprintf("%v", v)
		if strings.Contains(val, " ") || (len(val) > 0 && !isLikelyBase64(val)) {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Rule:     "secret-not-base64",
				Message:  fmt.Sprintf("Secret key %q value does not look base64-encoded", k),
				Fix:      "Base64 encode values: echo -n 'value' | base64",
			})
		}
	}

	_, hasData := obj["data"]
	_, hasStringData := obj["stringData"]
	if hasData && hasStringData {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Rule:     "secret-mixed-data",
			Message:  "Secret uses both data and stringData — use one consistently",
		})
	}

	return findings
}


func checkPVC(obj map[string]interface{}) []Finding {
	var findings []Finding
	spec := getMap(obj, "spec")
	if strVal(spec, "storageClassName") == "" {
		findings = append(findings, Finding{
			Severity: SeverityInfo,
			Rule:     "pvc-no-storage-class",
			Message:  "PVC has no storageClassName — uses cluster default storage class",
			Fix:      "Explicitly set storageClassName to avoid unexpected storage class selection",
		})
	}
	resources := getMap(spec, "resources")
	requests := getMap(resources, "requests")
	if strValI(requests, "storage") == "" {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Rule:     "pvc-no-storage-size",
			Message:  "PVC has no storage size request",
			Fix:      "Add: resources:\n  requests:\n    storage: 1Gi",
		})
	}
	return findings
}


func checkHPA(obj map[string]interface{}) []Finding {
	var findings []Finding
	spec := getMap(obj, "spec")
	minR := toInt(spec["minReplicas"])
	maxR := toInt(spec["maxReplicas"])
	if maxR == 0 {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Rule:     "hpa-no-max-replicas",
			Message:  "HPA maxReplicas is 0 — autoscaler will not work",
			Fix:      "Set maxReplicas to a reasonable value e.g. 10",
		})
	}
	if minR > maxR && maxR > 0 {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Rule:     "hpa-min-exceeds-max",
			Message:  fmt.Sprintf("HPA minReplicas (%d) > maxReplicas (%d)", minR, maxR),
			Fix:      "Ensure minReplicas <= maxReplicas",
		})
	}
	return findings
}


func checkNetworkPolicy(obj map[string]interface{}) []Finding {
	var findings []Finding
	spec := getMap(obj, "spec")
	podSelector := getMap(spec, "podSelector")
	if len(podSelector) == 0 || len(getMap(podSelector, "matchLabels")) == 0 {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Rule:     "network-policy-empty-selector",
			Message:  "NetworkPolicy has empty podSelector — applies to ALL pods in namespace",
			Fix:      "Add matchLabels to target specific pods, or confirm this is intentional",
		})
	}
	return findings
}


func calcScore(findings []Finding) int {
	score := 100
	for _, f := range findings {
		switch f.Severity {
		case SeverityError:
			score -= 20
		case SeverityWarning:
			score -= 8
		case SeverityInfo:
			score -= 2
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

func splitYAMLDocs(content string) []string {
	return strings.Split(content, "\n---")
}

func getMap(obj map[string]interface{}, key string) map[string]interface{} {
	if obj == nil {
		return nil
	}
	v, ok := obj[key]
	if !ok {
		return nil
	}
	m, _ := v.(map[string]interface{})
	return m
}

func strVal(obj map[string]interface{}, key string) string {
	if obj == nil {
		return ""
	}
	v, ok := obj[key]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func strValI(obj map[string]interface{}, key string) string {
	return strVal(obj, key)
}

func toInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		n, _ := strconv.Atoi(val)
		return n
	}
	return 0
}

func parseMemory(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	multipliers := map[string]int64{
		"Ki": 1024, "Mi": 1024 * 1024, "Gi": 1024 * 1024 * 1024,
		"K": 1000, "M": 1000 * 1000, "G": 1000 * 1000 * 1000,
	}
	for suffix, mul := range multipliers {
		if strings.HasSuffix(s, suffix) {
			n, err := strconv.ParseInt(s[:len(s)-len(suffix)], 10, 64)
			if err == nil {
				return n * mul
			}
		}
	}
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func isLikelyBase64(s string) bool {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/="
	for _, c := range s {
		if !strings.ContainsRune(chars, c) {
			return false
		}
	}
	return true
}

func FormatReport(r ManifestReport) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== %s/%s (score: %d/100) ===\n", r.Kind, r.Name, r.Score))
	if len(r.Findings) == 0 {
		sb.WriteString("  ✓ No issues found\n")
		return sb.String()
	}
	for _, f := range r.Findings {
		icon := map[Severity]string{
			SeverityError:   "✗",
			SeverityWarning: "⚠",
			SeverityInfo:    "ℹ",
		}[f.Severity]
		sb.WriteString(fmt.Sprintf("  %s [%s] %s\n", icon, f.Rule, f.Message))
		if f.Fix != "" {
			sb.WriteString(fmt.Sprintf("    Fix: %s\n", f.Fix))
		}
	}
	return sb.String()
}
