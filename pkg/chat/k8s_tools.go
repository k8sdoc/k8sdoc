package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/user/k8sdoc/pkg/kubernetes"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


type K8sTools struct {
	client *kubernetes.Client
}

func NewK8sTools(client *kubernetes.Client) *K8sTools {
	return &K8sTools{client: client}
}


func (t *K8sTools) ListPods(ctx context.Context, namespace, labelSelector string) (*TableData, string, error) {
	list, err := t.client.GetClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, "", err
	}

	table := &TableData{
		Title:   fmt.Sprintf("Pods in %s", nsLabel(namespace)),
		Headers: []string{"NAMESPACE", "NAME", "READY", "STATUS", "RESTARTS", "AGE", "NODE"},
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Pods in %s (%d total):\n", nsLabel(namespace), len(list.Items)))

	for _, p := range list.Items {
		ready, total := podReadyCount(p)
		restarts := podRestarts(p)
		age := age(p.CreationTimestamp.Time)
		row := TableRow{
			"NAMESPACE": p.Namespace,
			"NAME":      p.Name,
			"READY":     fmt.Sprintf("%d/%d", ready, total),
			"STATUS":    string(p.Status.Phase),
			"RESTARTS":  fmt.Sprintf("%d", restarts),
			"AGE":       age,
			"NODE":      p.Spec.NodeName,
		}
		table.Rows = append(table.Rows, row)
		sb.WriteString(fmt.Sprintf("  %s/%s  ready=%d/%d  status=%s  restarts=%d  age=%s  node=%s\n",
			p.Namespace, p.Name, ready, total, p.Status.Phase, restarts, age, p.Spec.NodeName))
	}
	return table, sb.String(), nil
}

func (t *K8sTools) GetPodLogs(ctx context.Context, namespace, podName, containerName string, lines int64) (string, error) {
	if lines == 0 {
		lines = 100
	}
	opts := &v1.PodLogOptions{TailLines: &lines}
	if containerName != "" {
		opts.Container = containerName
	}
	req := t.client.GetClient().CoreV1().Pods(namespace).GetLogs(podName, opts)
	result, err := req.DoRaw(ctx)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

func (t *K8sTools) DescribePod(ctx context.Context, namespace, podName string) (string, error) {
	pod, err := t.client.GetClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	events, _ := t.client.GetClient().CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName),
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== Pod: %s/%s ===\n", namespace, podName))
	sb.WriteString(fmt.Sprintf("Status: %s\n", pod.Status.Phase))
	sb.WriteString(fmt.Sprintf("Node: %s\n", pod.Spec.NodeName))
	sb.WriteString(fmt.Sprintf("IP: %s\n", pod.Status.PodIP))
	sb.WriteString(fmt.Sprintf("Age: %s\n", age(pod.CreationTimestamp.Time)))

	if len(pod.Labels) > 0 {
		sb.WriteString("Labels:\n")
		for k, v := range pod.Labels {
			sb.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
		}
	}

	sb.WriteString("Containers:\n")
	for _, cs := range pod.Status.ContainerStatuses {
		sb.WriteString(fmt.Sprintf("  %s: ready=%v restarts=%d\n", cs.Name, cs.Ready, cs.RestartCount))
		if cs.State.Waiting != nil {
			sb.WriteString(fmt.Sprintf("    Waiting: %s — %s\n", cs.State.Waiting.Reason, cs.State.Waiting.Message))
		}
		if cs.State.Terminated != nil {
			sb.WriteString(fmt.Sprintf("    Terminated: exit=%d reason=%s\n",
				cs.State.Terminated.ExitCode, cs.State.Terminated.Reason))
		}
	}

	for _, c := range pod.Spec.Containers {
		if c.Resources.Requests != nil || c.Resources.Limits != nil {
			sb.WriteString(fmt.Sprintf("  %s resources: requests=%v limits=%v\n",
				c.Name, c.Resources.Requests, c.Resources.Limits))
		}
	}

	sb.WriteString("Conditions:\n")
	for _, cond := range pod.Status.Conditions {
		sb.WriteString(fmt.Sprintf("  %s=%s", cond.Type, cond.Status))
		if cond.Reason != "" {
			sb.WriteString(fmt.Sprintf(" (%s: %s)", cond.Reason, cond.Message))
		}
		sb.WriteString("\n")
	}

	if events != nil && len(events.Items) > 0 {
		sb.WriteString("Events (recent):\n")
		start := 0
		if len(events.Items) > 10 {
			start = len(events.Items) - 10
		}
		for _, ev := range events.Items[start:] {
			sb.WriteString(fmt.Sprintf("  [%s] %s: %s\n",
				ev.Type, ev.Reason, ev.Message))
		}
	}
	return sb.String(), nil
}


func (t *K8sTools) ListDeployments(ctx context.Context, namespace string) (*TableData, string, error) {
	list, err := t.client.GetClient().AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}
	table := &TableData{
		Title:   fmt.Sprintf("Deployments in %s", nsLabel(namespace)),
		Headers: []string{"NAMESPACE", "NAME", "READY", "UP-TO-DATE", "AVAILABLE", "AGE"},
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Deployments in %s (%d total):\n", nsLabel(namespace), len(list.Items)))
	for _, d := range list.Items {
		desired := int32(1)
		if d.Spec.Replicas != nil {
			desired = *d.Spec.Replicas
		}
		row := TableRow{
			"NAMESPACE":  d.Namespace,
			"NAME":       d.Name,
			"READY":      fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, desired),
			"UP-TO-DATE": fmt.Sprintf("%d", d.Status.UpdatedReplicas),
			"AVAILABLE":  fmt.Sprintf("%d", d.Status.AvailableReplicas),
			"AGE":        age(d.CreationTimestamp.Time),
		}
		table.Rows = append(table.Rows, row)
		sb.WriteString(fmt.Sprintf("  %s/%s  ready=%d/%d  available=%d  age=%s\n",
			d.Namespace, d.Name, d.Status.ReadyReplicas, desired, d.Status.AvailableReplicas, age(d.CreationTimestamp.Time)))
	}
	return table, sb.String(), nil
}


func (t *K8sTools) ListServices(ctx context.Context, namespace string) (*TableData, string, error) {
	list, err := t.client.GetClient().CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}
	table := &TableData{
		Title:   fmt.Sprintf("Services in %s", nsLabel(namespace)),
		Headers: []string{"NAMESPACE", "NAME", "TYPE", "CLUSTER-IP", "EXTERNAL-IP", "PORTS", "AGE"},
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Services in %s (%d total):\n", nsLabel(namespace), len(list.Items)))
	for _, s := range list.Items {
		externalIP := "<none>"
		if len(s.Status.LoadBalancer.Ingress) > 0 {
			if s.Status.LoadBalancer.Ingress[0].IP != "" {
				externalIP = s.Status.LoadBalancer.Ingress[0].IP
			} else {
				externalIP = s.Status.LoadBalancer.Ingress[0].Hostname
			}
		}
		ports := formatPorts(s.Spec.Ports)
		row := TableRow{
			"NAMESPACE":   s.Namespace,
			"NAME":        s.Name,
			"TYPE":        string(s.Spec.Type),
			"CLUSTER-IP":  s.Spec.ClusterIP,
			"EXTERNAL-IP": externalIP,
			"PORTS":       ports,
			"AGE":         age(s.CreationTimestamp.Time),
		}
		table.Rows = append(table.Rows, row)
		sb.WriteString(fmt.Sprintf("  %s/%s  type=%s  clusterIP=%s  ports=%s\n",
			s.Namespace, s.Name, s.Spec.Type, s.Spec.ClusterIP, ports))
	}
	return table, sb.String(), nil
}


func (t *K8sTools) ListNodes(ctx context.Context) (*TableData, string, error) {
	list, err := t.client.GetClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}
	table := &TableData{
		Title:   "Cluster Nodes",
		Headers: []string{"NAME", "STATUS", "ROLES", "AGE", "VERSION", "CPU", "MEMORY"},
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Cluster nodes (%d total):\n", len(list.Items)))
	for _, n := range list.Items {
		status := nodeStatus(n)
		roles := nodeRoles(n)
		cpu := n.Status.Capacity.Cpu().String()
		mem := n.Status.Capacity.Memory().String()
		row := TableRow{
			"NAME":    n.Name,
			"STATUS":  status,
			"ROLES":   roles,
			"AGE":     age(n.CreationTimestamp.Time),
			"VERSION": n.Status.NodeInfo.KubeletVersion,
			"CPU":     cpu,
			"MEMORY":  mem,
		}
		table.Rows = append(table.Rows, row)
		sb.WriteString(fmt.Sprintf("  %s  status=%s  roles=%s  version=%s  cpu=%s  mem=%s\n",
			n.Name, status, roles, n.Status.NodeInfo.KubeletVersion, cpu, mem))
	}
	return table, sb.String(), nil
}


func (t *K8sTools) ListNamespaces(ctx context.Context) (*TableData, string, error) {
	list, err := t.client.GetClient().CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}
	table := &TableData{
		Title:   "Namespaces",
		Headers: []string{"NAME", "STATUS", "AGE"},
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Namespaces (%d total):\n", len(list.Items)))
	for _, ns := range list.Items {
		row := TableRow{
			"NAME":   ns.Name,
			"STATUS": string(ns.Status.Phase),
			"AGE":    age(ns.CreationTimestamp.Time),
		}
		table.Rows = append(table.Rows, row)
		sb.WriteString(fmt.Sprintf("  %s  status=%s  age=%s\n", ns.Name, ns.Status.Phase, age(ns.CreationTimestamp.Time)))
	}
	return table, sb.String(), nil
}


func (t *K8sTools) ListEvents(ctx context.Context, namespace string, warningsOnly bool) (*TableData, string, error) {
	fieldSelector := ""
	if warningsOnly {
		fieldSelector = "type=Warning"
	}
	list, err := t.client.GetClient().CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, "", err
	}
	table := &TableData{
		Title:   fmt.Sprintf("Events in %s", nsLabel(namespace)),
		Headers: []string{"TYPE", "REASON", "OBJECT", "MESSAGE", "COUNT", "AGE"},
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Events in %s (%d total):\n", nsLabel(namespace), len(list.Items)))

	start := 0
	if len(list.Items) > 30 {
		start = len(list.Items) - 30
	}
	for _, ev := range list.Items[start:] {
		obj := fmt.Sprintf("%s/%s", ev.InvolvedObject.Kind, ev.InvolvedObject.Name)
		row := TableRow{
			"TYPE":    ev.Type,
			"REASON":  ev.Reason,
			"OBJECT":  obj,
			"MESSAGE": truncate(ev.Message, 80),
			"COUNT":   fmt.Sprintf("%d", ev.Count),
			"AGE":     age(ev.LastTimestamp.Time),
		}
		table.Rows = append(table.Rows, row)
		sb.WriteString(fmt.Sprintf("  [%s] %s on %s: %s (count=%d)\n",
			ev.Type, ev.Reason, obj, ev.Message, ev.Count))
	}
	return table, sb.String(), nil
}


func (t *K8sTools) ListConfigMaps(ctx context.Context, namespace string) (*TableData, string, error) {
	list, err := t.client.GetClient().CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}
	table := &TableData{
		Title:   fmt.Sprintf("ConfigMaps in %s", nsLabel(namespace)),
		Headers: []string{"NAMESPACE", "NAME", "DATA", "AGE"},
	}
	var sb strings.Builder
	for _, cm := range list.Items {
		row := TableRow{
			"NAMESPACE": cm.Namespace,
			"NAME":      cm.Name,
			"DATA":      fmt.Sprintf("%d", len(cm.Data)),
			"AGE":       age(cm.CreationTimestamp.Time),
		}
		table.Rows = append(table.Rows, row)
		sb.WriteString(fmt.Sprintf("  %s/%s  keys=%d\n", cm.Namespace, cm.Name, len(cm.Data)))
	}
	return table, sb.String(), nil
}


func (t *K8sTools) ListPVCs(ctx context.Context, namespace string) (*TableData, string, error) {
	list, err := t.client.GetClient().CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}
	table := &TableData{
		Title:   fmt.Sprintf("PersistentVolumeClaims in %s", nsLabel(namespace)),
		Headers: []string{"NAMESPACE", "NAME", "STATUS", "VOLUME", "CAPACITY", "ACCESS MODES", "AGE"},
	}
	var sb strings.Builder
	for _, pvc := range list.Items {
		capacity := ""
		if pvc.Status.Capacity != nil {
			if c, ok := pvc.Status.Capacity[v1.ResourceStorage]; ok {
				capacity = c.String()
			}
		}
		row := TableRow{
			"NAMESPACE":    pvc.Namespace,
			"NAME":         pvc.Name,
			"STATUS":       string(pvc.Status.Phase),
			"VOLUME":       pvc.Spec.VolumeName,
			"CAPACITY":     capacity,
			"ACCESS MODES": formatAccessModes(pvc.Status.AccessModes),
			"AGE":          age(pvc.CreationTimestamp.Time),
		}
		table.Rows = append(table.Rows, row)
		sb.WriteString(fmt.Sprintf("  %s/%s  status=%s  capacity=%s\n",
			pvc.Namespace, pvc.Name, pvc.Status.Phase, capacity))
	}
	return table, sb.String(), nil
}


func (t *K8sTools) GetClusterSummary(ctx context.Context) (string, error) {
	nodes, err := t.client.GetClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	pods, err := t.client.GetClient().CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	nsList, err := t.client.GetClient().CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	deps, err := t.client.GetClient().AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("=== Cluster Summary ===\n")
	sb.WriteString(fmt.Sprintf("Nodes:       %d\n", len(nodes.Items)))
	sb.WriteString(fmt.Sprintf("Namespaces:  %d\n", len(nsList.Items)))
	sb.WriteString(fmt.Sprintf("Pods:        %d\n", len(pods.Items)))
	sb.WriteString(fmt.Sprintf("Deployments: %d\n", len(deps.Items)))

	ready, notReady := 0, 0
	for _, n := range nodes.Items {
		if nodeStatus(n) == "Ready" {
			ready++
		} else {
			notReady++
		}
	}
	sb.WriteString(fmt.Sprintf("Node health: %d Ready, %d NotReady\n", ready, notReady))

	running, pending, failed, other := 0, 0, 0, 0
	for _, p := range pods.Items {
		switch p.Status.Phase {
		case v1.PodRunning:
			running++
		case v1.PodPending:
			pending++
		case v1.PodFailed:
			failed++
		default:
			other++
		}
	}
	sb.WriteString(fmt.Sprintf("Pod health:  %d Running, %d Pending, %d Failed, %d Other\n",
		running, pending, failed, other))

	return sb.String(), nil
}


func podReadyCount(p v1.Pod) (ready, total int) {
	for _, cs := range p.Status.ContainerStatuses {
		total++
		if cs.Ready {
			ready++
		}
	}
	if total == 0 {
		total = len(p.Spec.Containers)
	}
	return
}

func podRestarts(p v1.Pod) int32 {
	var total int32
	for _, cs := range p.Status.ContainerStatuses {
		total += cs.RestartCount
	}
	return total
}

func nodeStatus(n v1.Node) string {
	for _, cond := range n.Status.Conditions {
		if cond.Type == v1.NodeReady {
			if cond.Status == v1.ConditionTrue {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}

func nodeRoles(n v1.Node) string {
	var roles []string
	for k := range n.Labels {
		if strings.HasPrefix(k, "node-role.kubernetes.io/") {
			roles = append(roles, strings.TrimPrefix(k, "node-role.kubernetes.io/"))
		}
	}
	if len(roles) == 0 {
		return "<none>"
	}
	return strings.Join(roles, ",")
}

func formatPorts(ports []v1.ServicePort) string {
	var parts []string
	for _, p := range ports {
		if p.NodePort != 0 {
			parts = append(parts, fmt.Sprintf("%d:%d/%s", p.Port, p.NodePort, p.Protocol))
		} else {
			parts = append(parts, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
		}
	}
	return strings.Join(parts, ",")
}

func formatAccessModes(modes []v1.PersistentVolumeAccessMode) string {
	var parts []string
	for _, m := range modes {
		parts = append(parts, string(m))
	}
	return strings.Join(parts, ",")
}

func nsLabel(ns string) string {
	if ns == "" {
		return "all namespaces"
	}
	return "namespace " + ns
}

func age(t time.Time) string {
	if t.IsZero() {
		return "<unknown>"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}


func (t *K8sTools) ListSecrets(ctx context.Context, namespace string) (*TableData, string, error) {
	list, err := t.client.GetClient().CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}
	table := &TableData{
		Title:   fmt.Sprintf("Secrets in %s", nsLabel(namespace)),
		Headers: []string{"NAMESPACE", "NAME", "TYPE", "KEYS", "AGE"},
	}
	var sb strings.Builder
	for _, s := range list.Items {
		row := TableRow{
			"NAMESPACE": s.Namespace,
			"NAME":      s.Name,
			"TYPE":      string(s.Type),
			"KEYS":      fmt.Sprintf("%d", len(s.Data)),
			"AGE":       age(s.CreationTimestamp.Time),
		}
		table.Rows = append(table.Rows, row)
		sb.WriteString(fmt.Sprintf("  %s/%s  type=%s  keys=%d\n", s.Namespace, s.Name, s.Type, len(s.Data)))
	}
	return table, sb.String(), nil
}

func (t *K8sTools) ListDaemonSets(ctx context.Context, namespace string) (*TableData, string, error) {
	list, err := t.client.GetClient().AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}
	table := &TableData{
		Title:   fmt.Sprintf("DaemonSets in %s", nsLabel(namespace)),
		Headers: []string{"NAMESPACE", "NAME", "DESIRED", "READY", "AGE"},
	}
	var sb strings.Builder
	for _, ds := range list.Items {
		row := TableRow{
			"NAMESPACE": ds.Namespace, "NAME": ds.Name,
			"DESIRED": fmt.Sprintf("%d", ds.Status.DesiredNumberScheduled),
			"READY":   fmt.Sprintf("%d", ds.Status.NumberReady),
			"AGE":     age(ds.CreationTimestamp.Time),
		}
		table.Rows = append(table.Rows, row)
		sb.WriteString(fmt.Sprintf("  %s/%s desired=%d ready=%d\n", ds.Namespace, ds.Name, ds.Status.DesiredNumberScheduled, ds.Status.NumberReady))
	}
	return table, sb.String(), nil
}

func (t *K8sTools) ListStatefulSets(ctx context.Context, namespace string) (*TableData, string, error) {
	list, err := t.client.GetClient().AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}
	table := &TableData{
		Title:   fmt.Sprintf("StatefulSets in %s", nsLabel(namespace)),
		Headers: []string{"NAMESPACE", "NAME", "READY", "AGE"},
	}
	var sb strings.Builder
	for _, ss := range list.Items {
		row := TableRow{
			"NAMESPACE": ss.Namespace, "NAME": ss.Name,
			"READY": fmt.Sprintf("%d/%d", ss.Status.ReadyReplicas, ss.Status.Replicas),
			"AGE":   age(ss.CreationTimestamp.Time),
		}
		table.Rows = append(table.Rows, row)
		sb.WriteString(fmt.Sprintf("  %s/%s ready=%d/%d\n", ss.Namespace, ss.Name, ss.Status.ReadyReplicas, ss.Status.Replicas))
	}
	return table, sb.String(), nil
}

func (t *K8sTools) ListJobs(ctx context.Context, namespace string) (*TableData, string, error) {
	list, err := t.client.GetClient().BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}
	table := &TableData{
		Title:   fmt.Sprintf("Jobs in %s", nsLabel(namespace)),
		Headers: []string{"NAMESPACE", "NAME", "STATUS", "COMPLETIONS", "AGE"},
	}
	var sb strings.Builder
	for _, j := range list.Items {
		status := "Running"
		if j.Status.CompletionTime != nil {
			status = "Complete"
		} else if j.Status.Failed > 0 {
			status = "Failed"
		}
		completions := fmt.Sprintf("%d/%d", j.Status.Succeeded, func() int32 {
			if j.Spec.Completions != nil { return *j.Spec.Completions }
			return 1
		}())
		row := TableRow{
			"NAMESPACE": j.Namespace, "NAME": j.Name,
			"STATUS": status, "COMPLETIONS": completions,
			"AGE": age(j.CreationTimestamp.Time),
		}
		table.Rows = append(table.Rows, row)
		sb.WriteString(fmt.Sprintf("  %s/%s status=%s\n", j.Namespace, j.Name, status))
	}
	return table, sb.String(), nil
}

func (t *K8sTools) ListIngresses(ctx context.Context, namespace string) (*TableData, string, error) {
	list, err := t.client.GetClient().NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}
	table := &TableData{
		Title:   fmt.Sprintf("Ingresses in %s", nsLabel(namespace)),
		Headers: []string{"NAMESPACE", "NAME", "CLASS", "HOSTS", "AGE"},
	}
	var sb strings.Builder
	for _, ing := range list.Items {
		var hosts []string
		for _, rule := range ing.Spec.Rules {
			hosts = append(hosts, rule.Host)
		}
		class := ""
		if ing.Spec.IngressClassName != nil {
			class = *ing.Spec.IngressClassName
		}
		row := TableRow{
			"NAMESPACE": ing.Namespace, "NAME": ing.Name,
			"CLASS": class, "HOSTS": strings.Join(hosts, ","),
			"AGE": age(ing.CreationTimestamp.Time),
		}
		table.Rows = append(table.Rows, row)
		sb.WriteString(fmt.Sprintf("  %s/%s hosts=%s\n", ing.Namespace, ing.Name, strings.Join(hosts, ",")))
	}
	return table, sb.String(), nil
}

func (t *K8sTools) GetRolloutStatus(ctx context.Context, namespace, name string) (string, error) {
	dep, err := t.client.GetClient().AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	desired := int32(1)
	if dep.Spec.Replicas != nil {
		desired = *dep.Spec.Replicas
	}
	sb.WriteString(fmt.Sprintf("Deployment %s/%s:\n", namespace, name))
	sb.WriteString(fmt.Sprintf("  Desired:   %d\n", desired))
	sb.WriteString(fmt.Sprintf("  Updated:   %d\n", dep.Status.UpdatedReplicas))
	sb.WriteString(fmt.Sprintf("  Ready:     %d\n", dep.Status.ReadyReplicas))
	sb.WriteString(fmt.Sprintf("  Available: %d\n", dep.Status.AvailableReplicas))
	for _, cond := range dep.Status.Conditions {
		sb.WriteString(fmt.Sprintf("  Condition: %s=%s (%s)\n", cond.Type, cond.Status, cond.Message))
	}
	return sb.String(), nil
}


func (t *K8sTools) ResolvePodName(ctx context.Context, _ string, partial string) (fullName, resolvedNS string) {

	list, err := t.client.GetClient().CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", ""
	}

	for _, p := range list.Items {
		if p.Name == partial {
			return p.Name, p.Namespace
		}
	}

	for _, p := range list.Items {
		if strings.HasPrefix(p.Name, partial+"-") || strings.HasPrefix(p.Name, partial) {
			return p.Name, p.Namespace
		}
	}

	for _, p := range list.Items {
		if strings.Contains(p.Name, partial) {
			return p.Name, p.Namespace
		}
	}

	return "", ""
}


func (t *K8sTools) ListPersistentVolumes(ctx context.Context) (*TableData, string, error) {
	list, err := t.client.GetClient().CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}
	table := &TableData{
		Title:   "PersistentVolumes (cluster-wide)",
		Headers: []string{"NAME", "CAPACITY", "ACCESS MODES", "RECLAIM POLICY", "STATUS", "CLAIM", "AGE"},
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("PersistentVolumes (%d total):\n", len(list.Items)))
	for _, pv := range list.Items {
		cap := ""
		if s, ok := pv.Spec.Capacity[v1.ResourceStorage]; ok {
			cap = s.String()
		}
		modes := formatAccessModes(pv.Spec.AccessModes)
		claim := ""
		if pv.Spec.ClaimRef != nil {
			claim = pv.Spec.ClaimRef.Namespace + "/" + pv.Spec.ClaimRef.Name
		}
		row := TableRow{
			"NAME": pv.Name, "CAPACITY": cap,
			"ACCESS MODES": modes, "RECLAIM POLICY": string(pv.Spec.PersistentVolumeReclaimPolicy),
			"STATUS": string(pv.Status.Phase), "CLAIM": claim,
			"AGE": age(pv.CreationTimestamp.Time),
		}
		table.Rows = append(table.Rows, row)
		sb.WriteString(fmt.Sprintf("  %s  capacity=%s  status=%s  claim=%s\n",
			pv.Name, cap, pv.Status.Phase, claim))
	}
	return table, sb.String(), nil
}

func (t *K8sTools) GetContainerRuntimeInfo(ctx context.Context) (string, error) {
	nodes, err := t.client.GetClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, node := range nodes.Items {
		sb.WriteString(fmt.Sprintf("Node: %s\n", node.Name))
		sb.WriteString(fmt.Sprintf("  Container Runtime: %s\n", node.Status.NodeInfo.ContainerRuntimeVersion))
		sb.WriteString(fmt.Sprintf("  Kernel: %s\n", node.Status.NodeInfo.KernelVersion))
		sb.WriteString(fmt.Sprintf("  OS: %s\n", node.Status.NodeInfo.OSImage))
		sb.WriteString(fmt.Sprintf("  Kubelet: %s\n", node.Status.NodeInfo.KubeletVersion))
		sb.WriteString(fmt.Sprintf("  kube-proxy: %s\n", node.Status.NodeInfo.KubeProxyVersion))
	}
	return sb.String(), nil
}
