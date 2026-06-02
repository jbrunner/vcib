// Package discovery provides Kubernetes pod discovery for Varnish backends.
package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Pod represents a Varnish pod discovered in Kubernetes.
type Pod struct {
	Name    string
	IP      string
	IsReady bool
}

// Discoverer discovers Varnish pods in a Kubernetes namespace.
type Discoverer struct {
	client         kubernetes.Interface
	namespace      string
	labelSelector  string
	cacheTTL       time.Duration
	podsDiscovered prometheus.Gauge

	mu         sync.Mutex
	cachedPods []Pod
	cachedAt   time.Time
}

// New creates a Discoverer. It uses the in-cluster config when running inside a
// pod, and falls back to ~/.kube/config for local development. Panics if
// neither config can be loaded.
func New(namespace, labelSelector string, cacheTTL time.Duration, podsDiscovered prometheus.Gauge,
) (*Discoverer, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(fmt.Sprintf("failed to load kubeconfig: %v", err))
		}
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &Discoverer{
		client:         client,
		namespace:      namespace,
		labelSelector:  labelSelector,
		cacheTTL:       cacheTTL,
		podsDiscovered: podsDiscovered,
	}, nil
}

// ListPods returns all pods matching the label selector with their readiness state.
// Results are cached for the configured TTL duration.
func (d *Discoverer) ListPods(ctx context.Context) ([]Pod, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.cachedPods != nil && time.Since(d.cachedAt) < d.cacheTTL {
		return d.cachedPods, nil
	}

	pods, err := d.client.CoreV1().Pods(d.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: d.labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	slog.Debug("pod list fetched", "pods", pods.Items)

	result := make([]Pod, 0, len(pods.Items))
	for i := range pods.Items {
		pod := pods.Items[i]
		ready := false
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				ready = true

				break
			}
		}
		result = append(result, Pod{Name: pod.Name, IP: pod.Status.PodIP, IsReady: ready})
	}

	d.cachedPods = result
	d.cachedAt = time.Now()

	return result, nil
}

// ListReadyPods returns only the ready pods matching the label selector.
func (d *Discoverer) ListReadyPods(ctx context.Context) ([]Pod, error) {
	all, err := d.ListPods(ctx)
	if err != nil {
		return nil, err
	}
	ready := make([]Pod, 0, len(all))
	for _, p := range all {
		if p.IsReady {
			ready = append(ready, p)
		}
	}

	d.podsDiscovered.Set(float64(len(ready)))

	return ready, nil
}
