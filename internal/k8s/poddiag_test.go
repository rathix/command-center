package k8s

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/rathix/command-center/internal/state"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func ptrStr(s string) *string { return &s }

func TestMostSevereReason(t *testing.T) {
	tests := []struct {
		name    string
		reasons []string
		want    string
	}{
		{"empty", nil, ""},
		{"single known", []string{"OOMKilled"}, "OOMKilled"},
		{"single unknown", []string{"RunContainerError"}, "RunContainerError"},
		{"crash beats image pull", []string{"ImagePullBackOff", "CrashLoopBackOff"}, "CrashLoopBackOff"},
		{"oom beats error", []string{"Error", "OOMKilled"}, "OOMKilled"},
		{"image pull beats error", []string{"Error", "ImagePullBackOff"}, "ImagePullBackOff"},
		{"crash beats all", []string{"Error", "OOMKilled", "ImagePullBackOff", "CrashLoopBackOff"}, "CrashLoopBackOff"},
		{"unknown stays if no ranked", []string{"RunContainerError", "CreateContainerError"}, "RunContainerError"},
		{"ranked beats unknown", []string{"RunContainerError", "Error"}, "Error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MostSevereReason(tt.reasons)
			if got != tt.want {
				t.Errorf("MostSevereReason(%v) = %q, want %q", tt.reasons, got, tt.want)
			}
		})
	}
}

func TestDiagFromPod(t *testing.T) {
	tests := []struct {
		name         string
		pod          *corev1.Pod
		wantReasons  []string
		wantRestarts int
	}{
		{
			name: "running pod no issues",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}, RestartCount: 0},
					},
				},
			},
			wantReasons:  nil,
			wantRestarts: 0,
		},
		{
			name: "waiting with CrashLoopBackOff",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							State:        corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
							RestartCount: 5,
						},
					},
				},
			},
			wantReasons:  []string{"CrashLoopBackOff"},
			wantRestarts: 5,
		},
		{
			name: "terminated with OOMKilled",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							State:        corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled"}},
							RestartCount: 3,
						},
					},
				},
			},
			wantReasons:  []string{"OOMKilled"},
			wantRestarts: 3,
		},
		{
			name: "multiple containers aggregate restarts",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}, RestartCount: 2},
						{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "Error"}}, RestartCount: 1},
					},
				},
			},
			wantReasons:  []string{"Error"},
			wantRestarts: 3,
		},
		{
			name: "init container failure",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					InitContainerStatuses: []corev1.ContainerStatus{
						{
							State:        corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}},
							RestartCount: 0,
						},
					},
				},
			},
			wantReasons:  []string{"ImagePullBackOff"},
			wantRestarts: 0,
		},
		{
			name: "dedup same reason across containers",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}, RestartCount: 2},
						{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}, RestartCount: 3},
					},
				},
			},
			wantReasons:  []string{"CrashLoopBackOff"},
			wantRestarts: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reasons, restarts := DiagFromPod(tt.pod)
			if len(reasons) != len(tt.wantReasons) {
				t.Fatalf("reasons = %v, want %v", reasons, tt.wantReasons)
			}
			for i := range reasons {
				if reasons[i] != tt.wantReasons[i] {
					t.Errorf("reasons[%d] = %q, want %q", i, reasons[i], tt.wantReasons[i])
				}
			}
			if restarts != tt.wantRestarts {
				t.Errorf("restarts = %d, want %d", restarts, tt.wantRestarts)
			}
		})
	}
}

func TestDiagFromPods(t *testing.T) {
	tests := []struct {
		name string
		pods []*corev1.Pod
		want *state.PodDiagnostic
	}{
		{
			name: "nil pods",
			pods: nil,
			want: nil,
		},
		{
			name: "all healthy",
			pods: []*corev1.Pod{
				{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
					{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}, RestartCount: 0},
				}}},
			},
			want: nil,
		},
		{
			name: "single unhealthy pod",
			pods: []*corev1.Pod{
				{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
					{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}, RestartCount: 5},
				}}},
			},
			want: &state.PodDiagnostic{Reason: ptrStr("CrashLoopBackOff"), RestartCount: 5},
		},
		{
			name: "multiple pods aggregate",
			pods: []*corev1.Pod{
				{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
					{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "Error"}}, RestartCount: 1},
				}}},
				{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
					{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "OOMKilled"}}, RestartCount: 2},
				}}},
			},
			want: &state.PodDiagnostic{Reason: ptrStr("OOMKilled"), RestartCount: 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DiagFromPods(tt.pods)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("got %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("got nil, want non-nil")
			}
			if got.RestartCount != tt.want.RestartCount {
				t.Errorf("RestartCount = %d, want %d", got.RestartCount, tt.want.RestartCount)
			}
			if (got.Reason == nil) != (tt.want.Reason == nil) {
				t.Fatalf("Reason nil mismatch: got %v, want %v", got.Reason, tt.want.Reason)
			}
			if got.Reason != nil && *got.Reason != *tt.want.Reason {
				t.Errorf("Reason = %q, want %q", *got.Reason, *tt.want.Reason)
			}
		})
	}
}

func TestPodDiagnosticQuerier_QueryForService(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("empty pod names returns nil", func(t *testing.T) {
		q := NewPodDiagnosticQuerier(fake.NewSimpleClientset(), logger)
		got := q.QueryForService(context.Background(), "default", nil)
		if got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})

	t.Run("pod not found returns nil", func(t *testing.T) {
		q := NewPodDiagnosticQuerier(fake.NewSimpleClientset(), logger)
		got := q.QueryForService(context.Background(), "default", []string{"nonexistent"})
		if got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})

	t.Run("healthy pod returns nil", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "app-abc", Namespace: "default"},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}, RestartCount: 0},
				},
			},
		}
		q := NewPodDiagnosticQuerier(fake.NewSimpleClientset(pod), logger)
		got := q.QueryForService(context.Background(), "default", []string{"app-abc"})
		if got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})

	t.Run("crashlooping pod returns diagnostic", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "app-abc", Namespace: "default"},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{
						State:        corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
						RestartCount: 5,
					},
				},
			},
		}
		q := NewPodDiagnosticQuerier(fake.NewSimpleClientset(pod), logger)
		got := q.QueryForService(context.Background(), "default", []string{"app-abc"})
		if got == nil {
			t.Fatal("got nil, want non-nil")
		}
		if got.Reason == nil || *got.Reason != "CrashLoopBackOff" {
			t.Errorf("Reason = %v, want CrashLoopBackOff", got.Reason)
		}
		if got.RestartCount != 5 {
			t.Errorf("RestartCount = %d, want 5", got.RestartCount)
		}
	})

	t.Run("mixed pods aggregate diagnostics", func(t *testing.T) {
		pod1 := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "app-1", Namespace: "default"},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}, RestartCount: 0},
				},
			},
		}
		pod2 := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "app-2", Namespace: "default"},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{
						State:        corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}},
						RestartCount: 1,
					},
				},
			},
		}
		q := NewPodDiagnosticQuerier(fake.NewSimpleClientset(pod1, pod2), logger)
		got := q.QueryForService(context.Background(), "default", []string{"app-1", "app-2"})
		if got == nil {
			t.Fatal("got nil, want non-nil")
		}
		if got.Reason == nil || *got.Reason != "ImagePullBackOff" {
			t.Errorf("Reason = %v, want ImagePullBackOff", got.Reason)
		}
		if got.RestartCount != 1 {
			t.Errorf("RestartCount = %d, want 1", got.RestartCount)
		}
	})
}
