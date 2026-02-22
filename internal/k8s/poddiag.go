package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/rathix/command-center/internal/state"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// severityRank maps known pod failure reasons to a numeric severity.
// Higher values are more severe.
var severityRank = map[string]int{
	"CrashLoopBackOff": 4,
	"OOMKilled":        3,
	"ImagePullBackOff": 2,
	"Error":            1,
}

// MostSevereReason returns the most severe reason from a list of reasons.
// If the list is empty, it returns an empty string.
func MostSevereReason(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	best := reasons[0]
	bestRank := severityRank[best] // 0 if unknown
	for _, r := range reasons[1:] {
		rank := severityRank[r]
		if rank > bestRank {
			best = r
			bestRank = rank
		}
	}
	return best
}

// DiagFromPod extracts diagnostic reasons and total restart count from a single pod.
func DiagFromPod(pod *corev1.Pod) (reasons []string, restartCount int) {
	allStatuses := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)
	seen := map[string]bool{}
	for _, cs := range allStatuses {
		restartCount += int(cs.RestartCount)
		if w := cs.State.Waiting; w != nil && w.Reason != "" {
			if !seen[w.Reason] {
				seen[w.Reason] = true
				reasons = append(reasons, w.Reason)
			}
		}
		if t := cs.State.Terminated; t != nil && t.Reason != "" {
			if !seen[t.Reason] {
				seen[t.Reason] = true
				reasons = append(reasons, t.Reason)
			}
		}
	}
	return reasons, restartCount
}

// DiagFromPods aggregates diagnostics across multiple pods into a single PodDiagnostic.
// Returns nil when there is nothing noteworthy to report.
func DiagFromPods(pods []*corev1.Pod) *state.PodDiagnostic {
	var allReasons []string
	totalRestarts := 0
	seen := map[string]bool{}
	for _, p := range pods {
		reasons, rc := DiagFromPod(p)
		totalRestarts += rc
		for _, r := range reasons {
			if !seen[r] {
				seen[r] = true
				allReasons = append(allReasons, r)
			}
		}
	}
	if len(allReasons) == 0 && totalRestarts == 0 {
		return nil
	}
	diag := &state.PodDiagnostic{RestartCount: totalRestarts}
	if best := MostSevereReason(allReasons); best != "" {
		diag.Reason = &best
	}
	return diag
}

// PodDiagnosticQuerier queries Kubernetes for pod-level diagnostics.
type PodDiagnosticQuerier struct {
	clientset kubernetes.Interface
	logger    *slog.Logger
}

// NewPodDiagnosticQuerier creates a new PodDiagnosticQuerier.
func NewPodDiagnosticQuerier(clientset kubernetes.Interface, logger *slog.Logger) *PodDiagnosticQuerier {
	return &PodDiagnosticQuerier{clientset: clientset, logger: logger}
}

// QueryForService fetches pods by name in the given namespace and returns diagnostics.
func (q *PodDiagnosticQuerier) QueryForService(ctx context.Context, namespace string, podNames []string) *state.PodDiagnostic {
	if len(podNames) == 0 {
		return nil
	}

	var mu sync.Mutex
	var pods []*corev1.Pod
	var wg sync.WaitGroup

	for _, name := range podNames {
		wg.Add(1)
		go func(podName string) {
			defer wg.Done()
			pod, err := q.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				q.logger.Warn("failed to get pod", "pod", podName, "namespace", namespace, "error", fmt.Sprintf("%v", err))
				return
			}
			mu.Lock()
			pods = append(pods, pod)
			mu.Unlock()
		}(name)
	}

	wg.Wait()
	return DiagFromPods(pods)
}
