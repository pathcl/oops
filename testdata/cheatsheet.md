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

### abs
Returns the absolute value of each sample — useful for signed metrics or deviation calculations.
```promql
abs(delta(cpu_usage[5m]))
```

### ceil
Rounds each sample up to the nearest integer — useful for conservative capacity estimates.
```promql
ceil(node_memory_MemFree_bytes / 1024 / 1024)
```

### floor
Rounds each sample down to the nearest integer — useful for floor-based resource calculations.
```promql
floor(node_memory_MemAvailable_bytes / 1024 / 1024)
```

### round
Rounds each sample to the nearest step — useful for cleaning up noisy decimal metrics.
```promql
round(rate(http_requests_total[5m]), 0.01)
```

### sqrt
Returns the square root of each sample — useful for converting variance to standard deviation.
```promql
sqrt(stdvar_over_time(response_latency_ms[5m]))
```

### exp
Returns e raised to the power of each sample — useful for reversing natural log transformations.
```promql
exp(ln_metric)
```

### ln
Returns the natural logarithm of each sample — useful for compressing wide-range metrics onto a log scale.
```promql
ln(process_resident_memory_bytes)
```

### log2
Returns the base-2 logarithm of each sample — useful for metrics that scale in powers of two.
```promql
log2(disk_io_operations_total)
```

### log10
Returns the base-10 logarithm of each sample — useful for displaying metrics on a log scale.
```promql
log10(http_requests_total)
```

### sgn
Returns the sign of each sample: 1 for positive, -1 for negative, 0 for zero — useful for detecting direction of change.
```promql
sgn(delta(container_memory_working_set_bytes[5m]))
```

### sin
Returns the sine of each sample value in radians — useful for modelling cyclic patterns.
```promql
sin(hour() * 0.2618)
```

### cos
Returns the cosine of each sample value in radians — useful for phase-shifted cyclic patterns.
```promql
cos(day_of_week() * 0.8976)
```

### tan
Returns the tangent of each sample value in radians.
```promql
tan(angle_radians)
```

### asin
Returns the arc sine of each sample — inverse of sin().
```promql
asin(normalized_ratio)
```

### acos
Returns the arc cosine of each sample — inverse of cos().
```promql
acos(normalized_value)
```

### atan
Returns the arc tangent of each sample — inverse of tan().
```promql
atan(slope_ratio)
```

### atan2
Returns the arc tangent of y/x using sign of both arguments — useful for 2D angle calculations between two vectors.
```promql
atan2(y_component, x_component)
```

### sinh
Returns the hyperbolic sine of each sample.
```promql
sinh(growth_metric)
```

### cosh
Returns the hyperbolic cosine of each sample.
```promql
cosh(scaled_metric)
```

### tanh
Returns the hyperbolic tangent of each sample — normalises values to the range (-1, 1).
```promql
tanh(cpu_usage_scaled)
```

### asinh
Returns the inverse hyperbolic sine of each sample.
```promql
asinh(stretched_metric)
```

### acosh
Returns the inverse hyperbolic cosine of each sample.
```promql
acosh(cosh_result)
```

### atanh
Returns the inverse hyperbolic tangent of each sample.
```promql
atanh(normalized_metric)
```

### deg
Converts radian values to degrees — useful for making angle metrics human-readable.
```promql
deg(rotation_radians)
```

### rad
Converts degree values to radians — useful for angle calculations.
```promql
rad(rotation_degrees)
```

### pi
Returns the mathematical constant π — useful for circular geometry calculations.
```promql
2 * pi() * radius_metric
```

### idelta
Calculates the difference between the last two samples in a range — instant delta for gauges.
```promql
idelta(disk_free_bytes[5m])
```

### histogram_count
Extracts the total observation count from a native histogram.
```promql
histogram_count(rate(http_request_duration_seconds[5m]))
```

### histogram_sum
Extracts the sum of all observations from a native histogram — divide by histogram_count for mean.
```promql
histogram_sum(rate(http_request_duration_seconds[5m]))
```

### histogram_stddev
Returns the estimated standard deviation from a native histogram.
```promql
histogram_stddev(rate(http_request_duration_seconds[5m]))
```

### histogram_stdvar
Returns the estimated variance from a native histogram.
```promql
histogram_stdvar(rate(http_request_duration_seconds[5m]))
```

### histogram_quantiles
Calculates multiple quantiles at once from a histogram — returns one series per quantile.
```promql
histogram_quantiles("quantile", rate(http_request_duration_seconds[5m]), 0.5, 0.95, 0.99)
```

### label_join
Joins multiple label values into a new label using a separator — useful for creating composite identifiers.
```promql
label_join(up, "host_port", ":", "instance", "port")
```

### time
Returns the current evaluation timestamp as a Unix epoch — useful for age and staleness calculations.
```promql
time() - process_start_time_seconds
```

### year
Extracts the year from a Unix timestamp.
```promql
year(vector(time()))
```

### month
Extracts the month (1–12) from a Unix timestamp.
```promql
month(vector(time()))
```

### day_of_month
Extracts the day of the month (1–31) from a Unix timestamp.
```promql
day_of_month(vector(time()))
```

### day_of_week
Extracts the day of the week (0=Sunday … 6=Saturday) from a Unix timestamp — useful for weekday/weekend splits.
```promql
day_of_week(vector(time()))
```

### day_of_year
Extracts the day of year (1–366) from a Unix timestamp.
```promql
day_of_year(vector(time()))
```

### days_in_month
Returns the number of days in the current month (28–31) — useful for normalising monthly totals.
```promql
increase(billing_events_total[1d]) * days_in_month(vector(time()))
```

### hour
Extracts the hour (0–23) from a Unix timestamp — useful for hourly traffic patterns and business-hours filtering.
```promql
hour(vector(time()))
```

### minute
Extracts the minute (0–59) from a Unix timestamp.
```promql
minute(vector(time()))
```

### scalar
Converts a single-element instant vector to a scalar — required when using a vector result as a scalar operand.
```promql
scalar(count(up{job="api"}))
```

### vector
Converts a scalar constant to a single-element instant vector — useful for threshold lines on dashboards.
```promql
vector(0.99)
```

### sort_by_label
Sorts instant vector elements by the value of a label in ascending order — useful for deterministic result ordering.
```promql
sort_by_label(up, "instance")
```

### sort_by_label_desc
Sorts instant vector elements by label value in descending order.
```promql
sort_by_label_desc(up, "job")
```

### limitk
Returns k pseudo-randomly selected elements — useful for sampling large result sets without bias.
```promql
limitk(10, up)
```

### limit_ratio
Returns a pseudo-random ratio of elements — useful for sampling a percentage of all series.
```promql
limit_ratio(0.1, http_requests_total)
```

### group
Returns 1 for each unique label combination in the input — collapses all values to 1, useful for set operations.
```promql
group(up)
```

### double_exponential_smoothing
Applies Holt's double exponential smoothing to a range vector — useful for smoothing metrics with a trend component.
```promql
double_exponential_smoothing(node_load1[1h], 0.3, 0.1)
```

### info
Joins labels from an info-type metric onto another metric — enriches metrics with metadata like version or region.
```promql
cpu_usage * on(instance) group_left(version, region)
  node_info{version=~".+"}
```

### start
Returns the start time of the query range as a Unix timestamp — available in range queries.
```promql
end() - start()
```

### end
Returns the end time of the query range as a Unix timestamp — available in range queries.
```promql
end() - start()
```

### range
Returns the duration of the query range in seconds — useful for normalising metrics over the query window.
```promql
increase(http_requests_total[1d]) / range()
```

### step
Returns the query resolution step interval in seconds.
```promql
rate(http_requests_total[1m]) / step()
```

### sum aggregation
Sums values across all series or within label groups — foundation of most PromQL aggregations.
```promql
sum by (job, namespace) (rate(http_requests_total[5m]))
```

### avg aggregation
Calculates the arithmetic mean across series or within label groups.
```promql
avg by (instance) (cpu_usage_percent)
```

### min aggregation
Selects the smallest value across series or within groups.
```promql
min by (region) (node_memory_MemAvailable_bytes)
```

### max aggregation
Selects the largest value across series or within groups.
```promql
max by (region) (node_memory_MemUsed_bytes)
```

### count aggregation
Counts the number of series in each group — useful for cardinality monitoring.
```promql
count by (namespace) (kube_pod_info)
```

### stddev aggregation
Calculates the population standard deviation across series in each group — high value means inconsistent behaviour across instances.
```promql
stddev by (job) (response_latency_ms)
```

### stdvar aggregation
Calculates the population variance across series in each group.
```promql
stdvar by (job) (cpu_usage_percent)
```

### quantile aggregation
Calculates a φ-quantile across series in each group — e.g. the p95 memory usage across all pods.
```promql
quantile by (namespace) (0.95, container_memory_working_set_bytes)
```

### Binary operator and
Intersection — returns elements from the left vector whose label sets also appear in the right vector.
```promql
up{job="api"} == 1 and rate(http_requests_total[5m]) > 100
```

### Binary operator or
Union — returns all elements from both vectors, preferring left-side values on label conflicts.
```promql
up{job="api"} or up{job="worker"}
```

### Binary operator unless
Complement — returns elements from the left vector whose label sets do NOT appear in the right vector.
```promql
all_pods unless maintenance_pods
```

### Histogram trim upper (</)
Removes observations above a threshold from a native histogram — useful for excluding outliers.
```promql
http_request_duration_seconds </ 10
```

### Histogram trim lower (>/)
Removes observations below a threshold from a native histogram — useful for filtering out near-zero noise.
```promql
http_request_duration_seconds >/ 0.001
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
