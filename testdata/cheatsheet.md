## PromQL

### Request rate by service
Day-to-day. Throughput (traffic golden signal) — requests per second per service over a 5m window.
```promql
sum(rate(http_requests_total[5m])) by (service)
```

### Error ratio
Day-to-day. Ratio of 5xx responses to total requests per service. Use as error budget signal.
```promql
sum(rate(http_requests_total{status=~"5.."}[5m])) by (service)
  / sum(rate(http_requests_total[5m])) by (service)
```

### P95 latency by service
Day-to-day. 95th-percentile request latency per service from histogram metrics.
```promql
histogram_quantile(0.95,
  sum(rate(http_request_duration_seconds_bucket[5m])) by (le, service)
)
```

### P99 latency by service
Day-to-day / SLO tracking. 99th-percentile latency — closer to worst-case user experience.
```promql
histogram_quantile(0.99,
  sum(rate(http_request_duration_seconds_bucket[5m])) by (le, service)
)
```

### Memory saturation per container
Day-to-day. Memory usage as a percentage of container limit. Alert threshold is typically 80%.
```promql
(container_memory_working_set_bytes{container!=""}
  / container_spec_memory_limit_bytes{container!=""}) * 100
```

### CPU usage per node
Day-to-day. Average CPU utilisation per node (100 minus idle time).
```promql
100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)
```

### 5xx error rate spike
Incident. Fires when the per-second 5xx rate exceeds 5% — use in alerting rules.
```promql
sum(rate(http_requests_total{status=~"5.."}[1m])) by (service)
  / sum(rate(http_requests_total[1m])) by (service) > 0.05
```

### P99 latency degradation vs 1h ago
Incident. Compares current P99 against the value from 1 hour ago; fires on 2x regression.
```promql
histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))
  > 2 * histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m] offset 1h))
```

### Pod restart count
Troubleshooting. Number of container restarts in the last hour — detects crash loops.
```promql
sum(increase(kube_pod_container_status_restarts_total[1h])) by (namespace, pod, container)
```

### Top pods by CPU
Troubleshooting. Top 10 containers by CPU usage — useful when hunting a runaway process.
```promql
topk(10,
  sum(rate(container_cpu_usage_seconds_total{container!=""}[5m])) by (namespace, pod, container)
)
```

### Disk space usage percent
Troubleshooting. Percentage of filesystem used per mount point. Alert at 85%.
```promql
(node_filesystem_size_bytes{fstype!~"tmpfs|overlay"}
  - node_filesystem_free_bytes{fstype!~"tmpfs|overlay"})
  / node_filesystem_size_bytes{fstype!~"tmpfs|overlay"} * 100
```

## LogQL

### Error log rate per service
Day-to-day. Rate of error-level lines per service per minute — baseline error trending metric.
```logql
sum by (service) (rate({namespace="production"} | json | level="error" [1m]))
```

### Log volume by severity
Day-to-day. Lines per second broken down by log level — early warning for error spikes.
```logql
sum by (level) (rate({namespace="production"} | json [1m]))
```

### Slow requests from access logs
Day-to-day. Requests taking longer than 1000ms extracted from JSON access logs.
```logql
{namespace="production", app="api"} | json | duration_ms > 1000
```

### 5xx error rate per endpoint
Day-to-day. Per-second rate of HTTP 5xx errors grouped by path — RED metric for dashboards.
```logql
sum by (path) (rate({app="api"} | json | status >= 500 [5m]))
```

### Fatal errors with stack traces
Incident. Fatal log lines that include a stacktrace field — first stop during an incident triage.
```logql
{namespace="production"} | json | level="fatal" | stacktrace != ""
```

### Connection refused or timeout errors
Incident. Log lines matching connectivity failure patterns — indicates downstream outage.
```logql
{namespace="production"} |~ "connection refused|connection timeout|ECONNRESET"
```

### OOMKilled events
Incident. Kubernetes OOM kill events surfaced in logs — signals memory limit is too low.
```logql
{namespace="production"} |= "OOMKilled"
```

### P95 latency from logs
Troubleshooting. 95th-percentile latency computed by unwrapping a numeric field from JSON logs.
```logql
quantile_over_time(0.95,
  {app="api"} | json | unwrap duration_ms [5m]
) by (path)
```

### Endpoint error breakdown
Troubleshooting. Endpoints with more than 1% error rate — precise target for deep-dive.
```logql
sum by (path) (rate({app="api"} | json | status >= 400 [5m])) > 0.01
```

### Authentication and authorisation failures
Troubleshooting. Auth failure rate per service — detects token expiry, misconfig, or attacks.
```logql
sum by (service) (
  rate({namespace="production"} |~ "unauthorized|forbidden" [5m])
)
```

### Retry and circuit-breaker activity
Troubleshooting. Count of retry attempts per service — indicates a degraded downstream dependency.
```logql
sum by (service) (count_over_time({namespace="production"} |= "retrying" [5m]))
```

## TraceQL

### Traces with errors
Day-to-day. All spans in an error state — core RED signal for trace-based dashboards.
```traceql
{ status = error }
```

### Traces by service
Day-to-day. All traces that include a span from a specific service.
```traceql
{ resource.service.name = "api" }
```

### Slow traces
Day-to-day / SLO. Root spans exceeding 1 second — user-facing latency violations.
```traceql
{ rootName = "GET /api/v1" && duration > 1s }
```

### Traces slower than threshold
Incident. Traces where total end-to-end duration exceeds 5 seconds — active slowness investigation.
```traceql
{ traceDuration > 5s }
```

### 5xx HTTP errors in traces
Incident. Spans with 5xx HTTP status codes — correlates with error-rate alert spikes.
```traceql
{ span.http.status_code >= 500 }
```

### Errors on a specific endpoint
Incident. Error spans on a particular URL path — scoped incident investigation.
```traceql
{ span.http.url =~ ".*api/payment.*" && status = error }
```

### Slow database queries
Troubleshooting. Database spans slower than 500ms — pinpoints query-level bottlenecks.
```traceql
{ span.db.system = "postgresql" && duration > 500ms }
```

### Latency from a service to its dependencies
Troubleshooting. Client-side spans from one service — measures outbound latency to each dependency.
```traceql
{ resource.service.name = "api" && span.kind = "client" }
```

### Traces with cache misses
Troubleshooting. Cache GET operations that resulted in a miss — detects cache invalidation issues.
```traceql
{ span.cache.operation = "get" && span.cache.hit = false }
```

### Timeout exceptions across services
Troubleshooting. Spans that recorded a TimeoutError exception — isolates flaky dependencies.
```traceql
{ event.exception.type = "TimeoutError" }
```
