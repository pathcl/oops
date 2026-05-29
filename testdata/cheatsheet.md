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

### Unique label values
Day-to-day. List all distinct values of a label across a metric — use to discover service names, namespaces, versions, or any label dimension.
```promql
count_values("version", build_info)
```

### Instant rate for spike detection
Incident. Per-second rate based on the last two data points only — more sensitive than rate() for sudden bursts.
```promql
irate(http_requests_total[5m])
```

### Total increase over a window
Day-to-day. Absolute count of events in a time window — useful for "how many requests in the last hour".
```promql
increase(http_requests_total[1h])
```

### Predict metric in N seconds
Troubleshooting / capacity. Linear regression forecast — predicts when disk will be full or memory exhausted.
```promql
predict_linear(node_filesystem_free_bytes[1h], 4 * 3600)
```

### Alert on missing metric
Incident. Returns 1 when a time series disappears — fires if a service stops emitting metrics.
```promql
absent(up{job="api"})
```

### Alert on metric gap
Incident. Returns 1 when a metric has had no samples for a window — detects scrape failures or dead exporters.
```promql
absent_over_time(up{job="api"}[5m])
```

### Detect flapping
Troubleshooting. Counts how many times a metric changed value — high count means instability or oscillation.
```promql
changes(service_status[1h]) > 5
```

### Detect counter resets
Troubleshooting. Counts how many times a counter wrapped — each reset usually means a process restart.
```promql
resets(process_start_time_seconds[24h])
```

### Average over time window
Day-to-day. Smooth a noisy gauge by averaging all samples in a window.
```promql
avg_over_time(container_memory_working_set_bytes[10m])
```

### Max over time window
Day-to-day. Peak value of a gauge in a window — useful for high-watermark alerting.
```promql
max_over_time(container_memory_working_set_bytes[30m])
```

### Quantile over time window
Day-to-day. Percentile of a gauge across its sample history — e.g. p95 CPU over the last hour.
```promql
quantile_over_time(0.95, container_cpu_usage_seconds_total[1h])
```

### Standard deviation over time
Troubleshooting. Measures variability of a metric — high stddev means erratic behaviour.
```promql
stddev_over_time(response_latency_ms[30m])
```

### Present over time
Incident. Returns 1 for each series that had at least one sample in the window — inverse of absent_over_time.
```promql
present_over_time(critical_service_health[5m])
```

### Histogram average latency
Day-to-day. Mean request duration directly from histogram buckets — cheaper than histogram_quantile for a rough average.
```promql
histogram_avg(rate(http_request_duration_seconds[5m]))
```

### Fraction of requests below SLO
Day-to-day / SLO. Percentage of requests completing under a target duration (e.g. 500ms).
```promql
histogram_fraction(0, 0.5, rate(http_request_duration_seconds[5m]))
```

### Clamp metric to valid range
Day-to-day. Enforce a min/max bound — prevents negative values or outliers from breaking dashboards.
```promql
clamp(cpu_usage_percent, 0, 100)
```

### Bottom K — least utilized
Troubleshooting. Find the N least busy instances — useful for identifying underloaded nodes or idle workers.
```promql
bottomk(5, rate(container_cpu_usage_seconds_total[5m]))
```

### Sort results ascending
Day-to-day. Order query results by value — useful in dashboards to rank from lowest to highest.
```promql
sort(rate(http_requests_total[5m]))
```

### Sort results descending
Day-to-day. Order query results from highest to lowest — most common for top-N dashboards.
```promql
sort_desc(rate(http_requests_total[5m]))
```

### Replace or extract label value
Day-to-day. Rewrite a label using regex — extract namespace from a pod name or normalise label formats.
```promql
label_replace(
  pod_cpu_usage,
  "namespace", "$1",
  "pod", "([^-]+)-.*"
)
```

### Join labels from an info metric
Day-to-day. Enrich a metric with metadata labels (version, region, owner) from a separate info series.
```promql
cpu_usage * on(instance) group_left(version)
  node_meta_info{version=~".+"}
```

### Rate of change (derivative)
Troubleshooting. Per-second derivative using linear regression — shows whether a gauge is rising or falling and how fast.
```promql
deriv(node_memory_MemFree_bytes[15m])
```

### Stale data detection
Incident. How long ago a metric was last updated — alerts when scrapers fall behind or targets go silent.
```promql
time() - timestamp(up{job="api"})
```

### Query during business hours only
Day-to-day. Filter alerts or dashboards to working hours — reduces noise on off-hours dashboards.
```promql
rate(http_requests_total[5m])
  and on() (hour() >= 8 and hour() < 18)
  and on() (day_of_week() >= 1 and day_of_week() <= 5)
```

### Count series cardinality
Day-to-day. How many active time series match a selector — useful for monitoring label explosion.
```promql
count(http_requests_total)
```

### Value distribution by label
Day-to-day. Count how many series have each unique label value — answers "what versions are running?".
```promql
count by (version) (kube_pod_labels{label_app="api"})
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
