package diagnostic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/user/k8sdoc/pkg/kubernetes"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RuntimeReport struct {
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	Findings  []Finding `json:"findings"`
	Summary   string    `json:"summary"`
	RawData   string    `json:"rawData"`
}

type RuntimeDiagnostics struct {
	client *kubernetes.Client
}

func NewRuntimeDiagnostics(client *kubernetes.Client) *RuntimeDiagnostics {
	return &RuntimeDiagnostics{client: client}
}


func (rd *RuntimeDiagnostics) DiagnoseNamespace(ctx context.Context, namespace string) ([]RuntimeReport, string, error) {
	var reports []RuntimeReport
	var allText strings.Builder

	podReports, podText, err := rd.diagnosePods(ctx, namespace)
	if err == nil {
		reports = append(reports, podReports...)
		allText.WriteString(podText)
	}

	depReports, depText, err := rd.diagnoseDeployments(ctx, namespace)
	if err == nil {
		reports = append(reports, depReports...)
		allText.WriteString(depText)
	}

	svcReports, svcText, err := rd.diagnoseServices(ctx, namespace)
	if err == nil {
		reports = append(reports, svcReports...)
		allText.WriteString(svcText)
	}

	nodeReports, nodeText, err := rd.diagnoseNodes(ctx)
	if err == nil {
		reports = append(reports, nodeReports...)
		allText.WriteString(nodeText)
	}

	pvcReports, pvcText, err := rd.diagnosePVCs(ctx, namespace)
	if err == nil {
		reports = append(reports, pvcReports...)
		allText.WriteString(pvcText)
	}

	eventText, _ := rd.getWarningEvents(ctx, namespace)
	if eventText != "" {
		allText.WriteString("\n=== WARNING EVENTS ===\n")
		allText.WriteString(eventText)
	}

	return reports, allText.String(), nil
}

func (rd *RuntimeDiagnostics) diagnosePods(ctx context.Context, namespace string) ([]RuntimeReport, string, error) {
	list, err := rd.client.GetClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}

	var reports []RuntimeReport
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== POD DIAGNOSTICS (%s) ===\n", nsLabel(namespace)))

	for _, pod := range list.Items {
		report := RuntimeReport{
			Kind:      "Pod",
			Name:      pod.Name,
			Namespace: pod.Namespace,
		}

		for _, cs := range pod.Status.ContainerStatuses {
			// CrashLoopBackOff
			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
				report.Findings = append(report.Findings, Finding{
					Severity: SeverityError,
					Rule:     "crashloopbackoff",
					Message:  fmt.Sprintf("Container %q is in CrashLoopBackOff (restarts: %d)", cs.Name, cs.RestartCount),
					Fix:      fmt.Sprintf("Check logs: kubectl logs %s -n %s -c %s --previous", pod.Name, pod.Namespace, cs.Name),
				})
			}

			if cs.State.Waiting != nil &&
				(cs.State.Waiting.Reason == "ImagePullBackOff" || cs.State.Waiting.Reason == "ErrImagePull") {
				report.Findings = append(report.Findings, Finding{
					Severity: SeverityError,
					Rule:     "image-pull-error",
					Message:  fmt.Sprintf("Container %q cannot pull image: %s", cs.Name, cs.State.Waiting.Message),
					Fix:      "Check image name/tag, registry credentials, and network access to registry",
				})
			}

			if cs.State.Terminated != nil && cs.State.Terminated.Reason == "OOMKilled" {
				report.Findings = append(report.Findings, Finding{
					Severity: SeverityError,
					Rule:     "oomkilled",
					Message:  fmt.Sprintf("Container %q was OOMKilled — ran out of memory", cs.Name),
					Fix:      "Increase memory limit in resources.limits.memory",
				})
			}
			if cs.LastTerminationState.Terminated != nil && cs.LastTerminationState.Terminated.Reason == "OOMKilled" {
				report.Findings = append(report.Findings, Finding{
					Severity: SeverityWarning,
					Rule:     "prev-oomkilled",
					Message:  fmt.Sprintf("Container %q was OOMKilled in previous run (restarts: %d)", cs.Name, cs.RestartCount),
					Fix:      "Increase memory limit — current limit is too low",
				})
			}

			if cs.RestartCount > 5 {
				report.Findings = append(report.Findings, Finding{
					Severity: SeverityWarning,
					Rule:     "high-restart-count",
					Message:  fmt.Sprintf("Container %q has restarted %d times — check application logs", cs.Name, cs.RestartCount),
					Fix:      fmt.Sprintf("kubectl logs %s -n %s -c %s --previous", pod.Name, pod.Namespace, cs.Name),
				})
			}

			if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
				exitCode := cs.State.Terminated.ExitCode
				exitMsg := exitCodeMeaning(exitCode)
				report.Findings = append(report.Findings, Finding{
					Severity: SeverityError,
					Rule:     "non-zero-exit",
					Message:  fmt.Sprintf("Container %q exited with code %d — %s", cs.Name, exitCode, exitMsg),
					Fix:      fmt.Sprintf("Check logs: kubectl logs %s -n %s -c %s", pod.Name, pod.Namespace, cs.Name),
				})
			}
		}

		if pod.Status.Phase == v1.PodPending {
			for _, cond := range pod.Status.Conditions {
				if cond.Type == v1.PodScheduled && cond.Status == v1.ConditionFalse {
					reason := "unknown scheduling failure"
					fix := "Check node capacity: kubectl describe nodes"
					if strings.Contains(cond.Message, "Insufficient memory") {
						reason = "Insufficient memory on all nodes"
						fix = "Reduce memory request or add more nodes"
					} else if strings.Contains(cond.Message, "Insufficient cpu") {
						reason = "Insufficient CPU on all nodes"
						fix = "Reduce CPU request or add more nodes"
					} else if strings.Contains(cond.Message, "node(s) had taint") {
						reason = "Node taint prevents scheduling"
						fix = "Add toleration or remove taint: kubectl taint node <node> <taint>-"
					} else if strings.Contains(cond.Message, "didn't match Pod's node affinity") {
						reason = "Node affinity mismatch"
						fix = "Check nodeSelector/affinity rules in pod spec"
					}
					report.Findings = append(report.Findings, Finding{
						Severity: SeverityError,
						Rule:     "unschedulable",
						Message:  fmt.Sprintf("Pod unschedulable: %s", reason),
						Fix:      fix,
					})
				}
			}

			if len(report.Findings) == 0 {
				report.Findings = append(report.Findings, Finding{
					Severity: SeverityWarning,
					Rule:     "pod-pending",
					Message:  "Pod is Pending — waiting to be scheduled or for images to pull",
					Fix:      fmt.Sprintf("kubectl describe pod %s -n %s", pod.Name, pod.Namespace),
				})
			}
		}


		if pod.Status.Phase == v1.PodRunning {
			for _, cond := range pod.Status.Conditions {
				if cond.Type == v1.PodReady && cond.Status == v1.ConditionFalse {
					report.Findings = append(report.Findings, Finding{
						Severity: SeverityWarning,
						Rule:     "pod-not-ready",
						Message:  fmt.Sprintf("Pod is Running but not Ready: %s — %s", cond.Reason, cond.Message),
						Fix:      "Check readinessProbe configuration and application startup",
					})
				}
			}
		}


		for _, c := range pod.Spec.Containers {
			if c.Resources.Limits == nil {
				report.Findings = append(report.Findings, Finding{
					Severity: SeverityWarning,
					Rule:     "no-resource-limits-runtime",
					Message:  fmt.Sprintf("Container %q has no resource limits — can consume unlimited CPU/memory", c.Name),
					Fix:      "Set resources.limits.cpu and resources.limits.memory",
				})
			}
		}

		status := string(pod.Status.Phase)
		age := ageDuration(pod.CreationTimestamp.Time)
		sb.WriteString(fmt.Sprintf("Pod %s/%s: status=%s age=%s restarts=%d findings=%d\n",
			pod.Namespace, pod.Name, status, age, totalRestarts(pod), len(report.Findings)))
		for _, f := range report.Findings {
			sb.WriteString(fmt.Sprintf("  [%s] %s — %s\n", f.Severity, f.Rule, f.Message))
		}

		if len(report.Findings) > 0 {
			report.Summary = fmt.Sprintf("%s/%s has %d issues", pod.Namespace, pod.Name, len(report.Findings))
			reports = append(reports, report)
		}
	}

	return reports, sb.String(), nil
}

func (rd *RuntimeDiagnostics) diagnoseDeployments(ctx context.Context, namespace string) ([]RuntimeReport, string, error) {
	list, err := rd.client.GetClient().AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}

	var reports []RuntimeReport
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n=== DEPLOYMENT DIAGNOSTICS (%s) ===\n", nsLabel(namespace)))

	for _, dep := range list.Items {
		report := RuntimeReport{
			Kind:      "Deployment",
			Name:      dep.Name,
			Namespace: dep.Namespace,
		}

		desired := int32(1)
		if dep.Spec.Replicas != nil {
			desired = *dep.Spec.Replicas
		}

		if dep.Status.AvailableReplicas < desired {
			report.Findings = append(report.Findings, Finding{
				Severity: SeverityError,
				Rule:     "deployment-unavailable",
				Message:  fmt.Sprintf("Only %d/%d replicas available", dep.Status.AvailableReplicas, desired),
				Fix:      fmt.Sprintf("kubectl describe deployment %s -n %s", dep.Name, dep.Namespace),
			})
		}

		if dep.Status.UpdatedReplicas < desired && dep.Status.UnavailableReplicas > 0 {
			report.Findings = append(report.Findings, Finding{
				Severity: SeverityWarning,
				Rule:     "deployment-rolling",
				Message:  fmt.Sprintf("Deployment rollout in progress: %d/%d updated", dep.Status.UpdatedReplicas, desired),
			})
		}

		for _, cond := range dep.Status.Conditions {
			if cond.Type == "Progressing" && cond.Reason == "ProgressDeadlineExceeded" {
				report.Findings = append(report.Findings, Finding{
					Severity: SeverityError,
					Rule:     "deployment-stalled",
					Message:  "Deployment rollout exceeded progress deadline — stuck",
					Fix:      "kubectl rollout undo deployment/" + dep.Name + " -n " + dep.Namespace,
				})
			}
		}

		sb.WriteString(fmt.Sprintf("Deployment %s/%s: desired=%d available=%d updated=%d findings=%d\n",
			dep.Namespace, dep.Name, desired, dep.Status.AvailableReplicas, dep.Status.UpdatedReplicas, len(report.Findings)))

		if len(report.Findings) > 0 {
			report.Summary = fmt.Sprintf("%s/%s rollout has issues", dep.Namespace, dep.Name)
			reports = append(reports, report)
		}
	}
	return reports, sb.String(), nil
}

func (rd *RuntimeDiagnostics) diagnoseServices(ctx context.Context, namespace string) ([]RuntimeReport, string, error) {
	epList, err := rd.client.GetClient().CoreV1().Endpoints(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}

	var reports []RuntimeReport
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n=== SERVICE DIAGNOSTICS (%s) ===\n", nsLabel(namespace)))

	for _, ep := range epList.Items {
		if ep.Name == "kubernetes" {
			continue
		}
		ready := 0
		for _, s := range ep.Subsets {
			ready += len(s.Addresses)
		}
		if ready == 0 {
			svc, err := rd.client.GetClient().CoreV1().Services(ep.Namespace).Get(ctx, ep.Name, metav1.GetOptions{})
			if err != nil || len(svc.Spec.Selector) == 0 {
				continue
			}
			var selectorParts []string
			for k, v := range svc.Spec.Selector {
				selectorParts = append(selectorParts, k+"="+v)
			}
			report := RuntimeReport{
				Kind:      "Service",
				Name:      ep.Name,
				Namespace: ep.Namespace,
				Findings: []Finding{{
					Severity: SeverityError,
					Rule:     "service-no-endpoints",
					Message:  fmt.Sprintf("Service %q has 0 ready endpoints — selector {%s} matches no running pods", ep.Name, strings.Join(selectorParts, ",")),
					Fix:      fmt.Sprintf("Check pod labels match service selector. Run: kubectl get pods -n %s -l %s", ep.Namespace, selectorParts[0]),
				}},
				Summary: fmt.Sprintf("Service %s/%s has no endpoints", ep.Namespace, ep.Name),
			}
			reports = append(reports, report)
			sb.WriteString(fmt.Sprintf("Service %s/%s: NO ENDPOINTS — selector=%v\n", ep.Namespace, ep.Name, svc.Spec.Selector))
		} else {
			sb.WriteString(fmt.Sprintf("Service %s/%s: %d ready endpoints — OK\n", ep.Namespace, ep.Name, ready))
		}
	}
	return reports, sb.String(), nil
}

func (rd *RuntimeDiagnostics) diagnoseNodes(ctx context.Context) ([]RuntimeReport, string, error) {
	list, err := rd.client.GetClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}

	var reports []RuntimeReport
	var sb strings.Builder
	sb.WriteString("\n=== NODE DIAGNOSTICS ===\n")

	for _, node := range list.Items {
		report := RuntimeReport{Kind: "Node", Name: node.Name}

		for _, cond := range node.Status.Conditions {
			switch cond.Type {
			case v1.NodeReady:
				if cond.Status != v1.ConditionTrue {
					report.Findings = append(report.Findings, Finding{
						Severity: SeverityError,
						Rule:     "node-not-ready",
						Message:  fmt.Sprintf("Node NotReady: %s — %s", cond.Reason, cond.Message),
						Fix:      "Check kubelet: ssh node && systemctl status kubelet",
					})
				}
			case v1.NodeMemoryPressure:
				if cond.Status == v1.ConditionTrue {
					report.Findings = append(report.Findings, Finding{
						Severity: SeverityError,
						Rule:     "node-memory-pressure",
						Message:  "Node has MemoryPressure — pods may be evicted",
						Fix:      "Free memory or add nodes. Check: kubectl top nodes",
					})
				}
			case v1.NodeDiskPressure:
				if cond.Status == v1.ConditionTrue {
					report.Findings = append(report.Findings, Finding{
						Severity: SeverityError,
						Rule:     "node-disk-pressure",
						Message:  "Node has DiskPressure — clean up images or expand disk",
						Fix:      "docker system prune OR expand disk volume",
					})
				}
			case v1.NodePIDPressure:
				if cond.Status == v1.ConditionTrue {
					report.Findings = append(report.Findings, Finding{
						Severity: SeverityError,
						Rule:     "node-pid-pressure",
						Message:  "Node has PIDPressure — too many processes",
						Fix:      "Check for runaway processes on the node",
					})
				}
			}
		}

		if node.Spec.Unschedulable {
			report.Findings = append(report.Findings, Finding{
				Severity: SeverityWarning,
				Rule:     "node-cordoned",
				Message:  fmt.Sprintf("Node %s is cordoned (unschedulable)", node.Name),
				Fix:      "kubectl uncordon " + node.Name,
			})
		}

		cpu := node.Status.Capacity.Cpu().String()
		mem := node.Status.Capacity.Memory().String()
		sb.WriteString(fmt.Sprintf("Node %s: cpu=%s mem=%s schedulable=%v findings=%d\n",
			node.Name, cpu, mem, !node.Spec.Unschedulable, len(report.Findings)))

		if len(report.Findings) > 0 {
			report.Summary = fmt.Sprintf("Node %s has issues", node.Name)
			reports = append(reports, report)
		}
	}
	return reports, sb.String(), nil
}

func (rd *RuntimeDiagnostics) diagnosePVCs(ctx context.Context, namespace string) ([]RuntimeReport, string, error) {
	list, err := rd.client.GetClient().CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}

	var reports []RuntimeReport
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n=== PVC DIAGNOSTICS (%s) ===\n", nsLabel(namespace)))

	for _, pvc := range list.Items {
		if pvc.Status.Phase == v1.ClaimPending {
			report := RuntimeReport{
				Kind:      "PersistentVolumeClaim",
				Name:      pvc.Name,
				Namespace: pvc.Namespace,
				Findings: []Finding{{
					Severity: SeverityError,
					Rule:     "pvc-pending",
					Message:  fmt.Sprintf("PVC %q is Pending — no matching PersistentVolume or StorageClass", pvc.Name),
					Fix:      "Check StorageClass exists: kubectl get storageclass",
				}},
				Summary: fmt.Sprintf("PVC %s/%s is stuck Pending", pvc.Namespace, pvc.Name),
			}
			reports = append(reports, report)
			sb.WriteString(fmt.Sprintf("PVC %s/%s: PENDING — no PV bound\n", pvc.Namespace, pvc.Name))
		} else {
			sb.WriteString(fmt.Sprintf("PVC %s/%s: %s — OK\n", pvc.Namespace, pvc.Name, pvc.Status.Phase))
		}
	}
	return reports, sb.String(), nil
}

func (rd *RuntimeDiagnostics) getWarningEvents(ctx context.Context, namespace string) (string, error) {
	list, err := rd.client.GetClient().CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "type=Warning",
	})
	if err != nil {
		return "", err
	}
	if len(list.Items) == 0 {
		return "", nil
	}

	var sb strings.Builder
	start := 0
	if len(list.Items) > 20 {
		start = len(list.Items) - 20
	}
	for _, ev := range list.Items[start:] {
		sb.WriteString(fmt.Sprintf("[%s] %s on %s/%s (count=%d): %s\n",
			ev.Reason,
			ev.InvolvedObject.Kind, ev.InvolvedObject.Namespace, ev.InvolvedObject.Name,
			ev.Count, ev.Message))
	}
	return sb.String(), nil
}


func exitCodeMeaning(code int32) string {
	switch code {
	case 1:
		return "general application error"
	case 2:
		return "misuse of shell command"
	case 126:
		return "command not executable"
	case 127:
		return "command not found"
	case 128:
		return "invalid exit argument"
	case 130:
		return "terminated by Ctrl+C"
	case 137:
		return "killed (OOMKilled or SIGKILL)"
	case 139:
		return "segmentation fault"
	case 143:
		return "graceful termination (SIGTERM)"
	default:
		return fmt.Sprintf("exit code %d", code)
	}
}

func totalRestarts(pod v1.Pod) int32 {
	var total int32
	for _, cs := range pod.Status.ContainerStatuses {
		total += cs.RestartCount
	}
	return total
}

func nsLabel(ns string) string {
	if ns == "" {
		return "all namespaces"
	}
	return ns
}

func ageDuration(t time.Time) string {
	if t.IsZero() {
		return "unknown"
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
