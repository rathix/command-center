package logtail

import (
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// K8sStreamer implements PodLogStreamer using the Kubernetes client-go API.
type K8sStreamer struct {
	clientset kubernetes.Interface
}

// NewK8sStreamer creates a new K8sStreamer from a Kubernetes clientset.
func NewK8sStreamer(clientset kubernetes.Interface) *K8sStreamer {
	return &K8sStreamer{clientset: clientset}
}

// StreamPodLogs opens a log stream for the specified pod.
func (s *K8sStreamer) StreamPodLogs(ctx context.Context, namespace, pod string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
	req := s.clientset.CoreV1().Pods(namespace).GetLogs(pod, opts)
	return req.Stream(ctx)
}

// GetPod retrieves the pod resource to check its current status.
func (s *K8sStreamer) GetPod(ctx context.Context, namespace, pod string) (*corev1.Pod, error) {
	return s.clientset.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
}
