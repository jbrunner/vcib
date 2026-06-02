# Varnish Cache Invalidation Broker Helm Chart

Deploys the [Varnish Cache Invalidation Broker](https://github.com/jbrunner/vcib/) vcib to Kubernetes.

## Prerequisites

- Kubernetes 1.21+
- Helm 3.x
- Varnish pods running in the target namespace, labelled with `app=varnish` (or a custom selector)

## Installation

```bash
helm install vcib oci://ghcr.io/jbrunner/vcib --namespace vcib --create-namespace
```

Override values inline:

```bash
helm install vcib oci://ghcr.io/jbrunner/vcib \
  --namespace vcib --create-namespace \
  --set config.varnishNamespace=production \
  --set config.clientAuth="Bearer mysecret"
```

## Configuration

| Key | Env var | Description | Default |
|---|---|---|---|
| `replicaCount` | | Number of VCIB replicas | `2` |
| `image.repository` | | Container image repository | `ghcr.io/jbrunner/vcib` |
| `image.tag` | | Image tag. Defaults to `.Chart.AppVersion` when empty. | `""` |
| `image.pullPolicy` | | Image pull policy | `IfNotPresent` |
| `config.varnishNamespace` | `VCIB_VARNISH_NAMESPACE` | Kubernetes namespace of the Varnish pods | `default` |
| `config.varnishLabelSelector` | `VCIB_VARNISH_LABEL_SELECTOR` | Label selector for Varnish pods | `app=varnish` |
| `config.varnishPort` | `VCIB_VARNISH_PORT` | Port of the Varnish instances | `6081` |
| `config.listenAddr` | `VCIB_LISTEN_ADDR` | Address VCIB listens on for invalidation requests | `:8080` |
| `config.metricsAddr` | `VCIB_METRICS_ADDR` | Address for `/metrics` and `/healthz` | `:9090` |
| `config.retryInterval` | `VCIB_RETRY_INTERVAL` | Wait time between retries | `10s` |
| `config.retryCount` | `VCIB_RETRY_COUNT` | Number of retries per pod before marking as failed | `3` |
| `config.requestTimeout` | `VCIB_REQUEST_TIMEOUT` | HTTP timeout for a single forwarding attempt to a Varnish pod | `5s` |
| `config.podCacheTTL` | `VCIB_POD_CACHE_TTL` | How long the pod list is cached before re-querying the API | `1s` |
| `config.forwardHeaders` | `VCIB_FORWARD_HEADERS` | Comma-separated request headers to forward to Varnish | `Host` |
| `config.maxConcurrentDispatches` | `VCIB_MAX_CONCURRENT_DISPATCHES` | Maximum number of concurrent pod dispatch goroutines | `500` |
| `config.logLevel` | `VCIB_LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) | `info` |
| `config.clientAuth` | `VCIB_CLIENT_AUTH` | Full `Authorization` header value clients must send (e.g. `Bearer secret`). Empty = disabled. | `""` |
| `config.varnishAuth` | `VCIB_VARNISH_AUTH` | Full `Authorization` header value VCIB sends to Varnish backends. Empty = disabled. | `""` |
| `rbac.create` | | Create `Role` and `RoleBinding` for pod discovery | `true` |
| `serviceAccount.create` | | Create a dedicated `ServiceAccount` | `true` |
| `serviceMonitor.enabled` | | Create a Prometheus Operator `ServiceMonitor` | `false` |
| `networkPolicy.enabled` | | Create `NetworkPolicy` resources | `false` |
