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

### Is traffic unusual for this time of day
Troubleshooting. Compares current request rate against the same 5-minute slot last week — catches anomalies that absolute thresholds miss.
```promql
rate(http_requests_total[5m])
  / avg_over_time(rate(http_requests_total[5m])[1w:5m] offset 1w)
```

### Is error rate higher than it was an hour ago
Incident. Ratio of current error rate to the rate 1 hour ago — catches degradations still within "normal" absolute numbers.
```promql
rate(http_requests_total{status=~"5.."}[5m])
  / rate(http_requests_total{status=~"5.."}[5m] offset 1h)
```

### Is memory growing and when will it run out
Troubleshooting / capacity. Predicts when each container will exhaust its memory limit based on the last hour's growth trend.
```promql
predict_linear(container_memory_working_set_bytes[1h], 3600)
  > container_spec_memory_limit_bytes * 0.9
```

### Which container has the most erratic memory usage
Troubleshooting. High stddev over time means unstable memory — good indicator of a slow leak or periodic spike.
```promql
sort_desc(
  stddev_over_time(container_memory_working_set_bytes[30m])
)
```

### Which services are up AND receiving traffic
Incident. Intersection of liveness and traffic — filters out services that are technically up but not serving requests.
```promql
up{job="api"} == 1
  and on(instance, job) rate(http_requests_total[5m]) > 0
```

### Which pods are alerting but NOT in maintenance
Incident. Excludes pods in a maintenance set so you focus on unexpected failures only.
```promql
kube_pod_status_ready{condition="false"}
  unless on(pod, namespace) maintenance_pods
```

### What percentage of pods in each deployment are ready
Day-to-day. Fraction of ready pods per deployment — faster to read than raw counts during an incident.
```promql
sum by (deployment) (kube_pod_status_ready{condition="true"})
  / sum by (deployment) (kube_pod_info)
```

### Which namespace is consuming the most memory
Incident / capacity. Ranks namespaces by total memory — first stop when a node is under memory pressure.
```promql
sort_desc(
  sum by (namespace) (container_memory_working_set_bytes{container!=""})
)
```

### Which instance is the latency outlier in the fleet
Troubleshooting. Flags instances whose p99 is more than double the fleet-wide p99.
```promql
histogram_quantile(0.99, sum by (instance, le) (rate(http_request_duration_seconds_bucket[5m])))
  > 2 * histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket[5m])))
```

### P50 P95 P99 latency side by side for all services
Day-to-day. Three percentiles in one query for a complete latency distribution view.
```promql
histogram_quantiles(
  "quantile",
  sum by (service, le) (rate(http_request_duration_seconds_bucket[5m])),
  0.5, 0.95, 0.99
)
```

### Which endpoints have the most inconsistent latency
Troubleshooting. High histogram standard deviation means some requests are much slower than others on the same endpoint.
```promql
sort_desc(
  histogram_stddev(
    sum by (service, le) (rate(http_request_duration_seconds_bucket[5m]))
  )
)
```

### What percentage of requests completed within SLO
Day-to-day / SLO. Fraction of requests finishing under 500ms — direct SLO compliance signal.
```promql
histogram_fraction(0, 0.5, rate(http_request_duration_seconds[5m]))
```

### Show error rate with team ownership labels
Day-to-day. Enriches error rate with team name from a service info metric — makes dashboards actionable without looking up ownership separately.
```promql
rate(http_requests_total{status=~"5.."}[5m])
  * on(service) group_left(team, owner)
  service_info{environment="production"}
```

### Is disk write load accelerating
Troubleshooting. The derivative of disk writes — a rising value means write pressure is increasing, not just constant.
```promql
deriv(node_disk_written_bytes_total[15m])
```

### How long has this process been running
Day-to-day. Process uptime in hours — confirms a rollout restarted the right instances.
```promql
(time() - process_start_time_seconds) / 3600
```

### Error rate on weekdays vs weekends
Troubleshooting. Restricts the query to weekday traffic only — isolates issues that only appear under business-hours load.
```promql
rate(http_requests_total{status=~"5.."}[5m])
  and on() (day_of_week() >= 1 and day_of_week() <= 5)
```

### Which scrape targets have been missing data
Incident. Counts targets per job that had no samples in the last 2 minutes — a gap means stale data and silent failures.
```promql
count by (job) (absent_over_time(up[2m]))
```

### How many distinct versions are running in production
Day-to-day. More than one means a rollout is in progress or a previous version was not fully replaced.
```promql
count(count by (version) (kube_pod_labels{namespace="production"}))
```

### Is load smooth or spiky
Troubleshooting. Compares the smoothed (double exponential) request rate against the raw rate — a large gap means the load is bursty, not steady.
```promql
double_exponential_smoothing(rate(http_requests_total[10m])[1h:], 0.3, 0.1)
```

### How many total requests happened in this time window
Day-to-day. Absolute count from a histogram when you need a number rather than a per-second rate.
```promql
histogram_count(rate(http_request_duration_seconds[5m]))
```

### Which services saw a counter reset (likely restart)
Incident. Each counter reset means a process restarted — non-zero resets in the last hour means something crashed.
```promql
resets(process_start_time_seconds[1h]) > 0
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
