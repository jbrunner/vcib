package discovery_test

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jbrunner/vcib/internal/discovery"
)

const (
	testNamespace    = "default"
	testLabelApp     = "app"
	testLabelVarnish = "varnish"
)

func makePod(name, podIP string, ready bool) corev1.Pod {
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}

	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels:    map[string]string{testLabelApp: testLabelVarnish},
		},
		Status: corev1.PodStatus{
			PodIP: podIP,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: status},
			},
		},
	}
}

func TestListPods_Empty(t *testing.T) {
	disc := discovery.NewTestDiscoverer(t)

	pods, err := disc.ListPods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pods) != 0 {
		t.Errorf("expected 0 pods, got %d", len(pods))
	}
}

func TestListPods_ReturnsAllPods(t *testing.T) {
	disc := discovery.NewTestDiscoverer(t,
		makePod("pod-a", "10.0.0.1", true),
		makePod("pod-b", "10.0.0.2", false),
	)

	pods, err := disc.ListPods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pods) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(pods))
	}
}

func TestListPods_ReadinessMapping(t *testing.T) {
	disc := discovery.NewTestDiscoverer(t,
		makePod("ready-pod", "10.0.0.1", true),
		makePod("unready-pod", "10.0.0.2", false),
	)

	pods, err := disc.ListPods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := make(map[string]discovery.Pod, len(pods))
	for _, pod := range pods {
		byName[pod.Name] = pod
	}

	if !byName["ready-pod"].IsReady {
		t.Error("ready-pod should be ready")
	}

	if byName["unready-pod"].IsReady {
		t.Error("unready-pod should not be ready")
	}
}

func TestListPods_IPAndName(t *testing.T) {
	disc := discovery.NewTestDiscoverer(t, makePod("my-pod", "192.168.1.5", true))

	pods, err := disc.ListPods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pods[0].Name != "my-pod" {
		t.Errorf("Name = %q, want %q", pods[0].Name, "my-pod")
	}

	if pods[0].IP != "192.168.1.5" {
		t.Errorf("IP = %q, want %q", pods[0].IP, "192.168.1.5")
	}
}

func TestListPods_NoPodReadyCondition(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-condition-pod",
			Namespace: testNamespace,
			Labels:    map[string]string{testLabelApp: testLabelVarnish},
		},
		Status: corev1.PodStatus{PodIP: "10.0.0.9"},
	}
	disc := discovery.NewTestDiscoverer(t, pod)

	pods, err := disc.ListPods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pods[0].IsReady {
		t.Error("pod without Ready condition should not be ready")
	}
}

func TestListReadyPods_FiltersUnready(t *testing.T) {
	disc := discovery.NewTestDiscoverer(t,
		makePod("ready-a", "10.0.0.1", true),
		makePod("ready-b", "10.0.0.2", true),
		makePod("unready-c", "10.0.0.3", false),
	)

	pods, err := disc.ListReadyPods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pods) != 2 {
		t.Errorf("expected 2 ready pods, got %d", len(pods))
	}

	for _, pod := range pods {
		if !pod.IsReady {
			t.Errorf("pod %q should be ready", pod.Name)
		}
	}
}

func TestListReadyPods_Empty(t *testing.T) {
	disc := discovery.NewTestDiscoverer(t,
		makePod("unready-a", "10.0.0.1", false),
		makePod("unready-b", "10.0.0.2", false),
	)

	pods, err := disc.ListReadyPods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pods) != 0 {
		t.Errorf("expected 0 ready pods, got %d", len(pods))
	}
}

func TestListReadyPods_UpdatesGauge(t *testing.T) {
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vcib_test_pods_ready",
		Help: "Number of ready pods (gauge update test).",
	})

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: testNamespace,
			Labels:    map[string]string{testLabelApp: testLabelVarnish},
		},
		Status: corev1.PodStatus{
			PodIP: "10.0.0.1",
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}

	disc := discovery.NewTestDiscovererWithGauge(t, gauge, pod)

	_, err := disc.ListReadyPods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(gauge)

	metricFamilies, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	if len(metricFamilies) == 0 {
		t.Fatal("no metric families gathered")
	}

	got := metricFamilies[0].GetMetric()[0].GetGauge().GetValue()
	if got != 1 {
		t.Errorf("gauge = %v, want 1", got)
	}
}
