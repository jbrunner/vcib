# Varnish Cache Invalidation Broker

Varnish Cache Invalidation Broker (VCIB) is a lightweight broker that receives `PURGE` and `BAN` requests from clients and forwards them asynchronously to all ready Varnish pods in a Kubernetes namespace. Clients speak plain Varnish HTTP. No custom protocol needed.

## How it works

1. A client sends `PURGE /path` or `BAN /path` with the correct `Host` header.
2. VCIB responds immediately with `202 Accepted`.
3. In the background, VCIB lists all ready Varnish pods and dispatches the request to each one concurrently.
4. Failed dispatches are retried every `RETRY_INTERVAL` up to `RETRY_COUNT` times.
5. Outcomes are exposed via Prometheus metrics.

## Installation

```bash
helm install vcib oci://ghcr.io/jbrunner/vcib --namespace vcib --create-namespace
```

See [chart/README.md](chart/README.md) for all Helm values.

## Usage

```bash
# Purge a single path
curl -X PURGE http://vcib:8080/path/to/resource \
  -H "Host: example.com"

# Ban by path prefix
curl -X BAN http://vcib:8080/images/ \
  -H "Host: example.com"

# With authentication (VCIB_CLIENT_AUTH set)
curl -X PURGE http://vcib:8080/path/to/resource \
  -H "Host: example.com" \
  -H "Authorization: Bearer mysecret"
```

## Configuration

| Variable | Description | Default |
|---|---|---|
| `VCIB_CLIENT_AUTH` | Full `Authorization` header value clients must send to VCIB (e.g. `Bearer secret`). Empty = disabled. | `` |
| `VCIB_FORWARD_HEADERS` | Comma-separated list of request headers to forward to varnish (supports wildcards, e.g. `Host,X-*`) | `Host` |
| `VCIB_LISTEN_ADDR` | Address VCIB listens on for invalidation requests | `:8080` |
| `VCIB_LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) | `info` |
| `VCIB_MAX_CONCURRENT_DISPATCHES` | Maximum number of concurrent pod dispatch goroutines (semaphore) | `500` |
| `VCIB_METRICS_ADDR` | Address for `/metrics` and `/healthz` | `:9090` |
| `VCIB_POD_CACHE_TTL` | How long the Kubernetes pod list is cached before re-querying the API | `1s` |
| `VCIB_REQUEST_TIMEOUT` | HTTP timeout for a single forwarding attempt to a Varnish pod | `5s` |
| `VCIB_RETRY_COUNT` | Number of retries per pod before marking as failed | `3` |
| `VCIB_RETRY_INTERVAL` | Wait time between retries | `10s` |
| `VCIB_VARNISH_AUTH` | Full `Authorization` header value VCIB sends to Varnish backends (e.g. `Bearer foo`, `Basic foo:bar`). Empty = disabled. | `` |
| `VCIB_VARNISH_LABEL_SELECTOR` | Label selector for Varnish pods | `app=varnish` |
| `VCIB_VARNISH_NAMESPACE` | Kubernetes namespace of the Varnish pods | `default` |
| `VCIB_VARNISH_PORT` | Port of the Varnish instances | `6081` |

## Metrics

| Metric | Type | Labels |
|---|---|---|
| `vcib_invalidation_requests_total` | Counter | `method` |
| `vcib_pod_confirmations_total` | Counter | `pod`, `status` |
| `vcib_retries_total` | Counter | `pod` |
| `vcib_invalidation_duration_seconds` | Histogram | `method` |
| `vcib_pods_discovered` | Gauge | – |
| `vcib_dispatch_concurrency_limit` | Gauge | – |

A Grafana dashboard is available at [grafana/dashboard.json](grafana/dashboard.json).

## High availability

Multiple VCIB replicas can run in parallel. Each instance receives requests independently via the Kubernetes Service; no coordination between replicas is needed.
