package discovery

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

// NewTestDiscoverer creates a Discoverer backed by a fake Kubernetes client for testing.
// It uses namespace "default" and label selector "app=varnish".
func NewTestDiscoverer(t *testing.T, pods ...corev1.Pod) *Discoverer {
	t.Helper()

	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vcib_test_pods_discovered",
		Help: "Number of ready Varnish pods (test instance).",
	})

	return newTestDiscoverer(t, gauge, pods...)
}

// NewTestDiscovererWithGauge creates a Discoverer that updates the given gauge.
func NewTestDiscovererWithGauge(t *testing.T, gauge prometheus.Gauge, pods ...corev1.Pod) *Discoverer {
	t.Helper()

	return newTestDiscoverer(t, gauge, pods...)
}

func newTestDiscoverer(t *testing.T, gauge prometheus.Gauge, pods ...corev1.Pod) *Discoverer {
	t.Helper()

	objects := make([]runtime.Object, len(pods))
	for idx := range pods {
		objects[idx] = &pods[idx]
	}

	return &Discoverer{
		client:         fake.NewClientset(objects...),
		namespace:      "default",
		labelSelector:  "app=varnish",
		podsDiscovered: gauge,
	}
}
