# SRE Query Cheatsheet

## PromQL

### Rate of requests
Per-second rate of HTTP requests averaged over a 5m window.
```promql
rate(http_requests_total[5m])
```

### Error ratio
Ratio of 5xx errors to total requests.
```promql
sum(rate(http_requests_total{status=~"5.."}[5m]))
/ sum(rate(http_requests_total[5m]))
```

### P99 latency
99th percentile request latency.
```promql
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))
```

### CPU usage by pod
CPU usage across all pods in a namespace.
```promql
sum(rate(container_cpu_usage_seconds_total{namespace="production"}[5m])) by (pod)
```

## LogQL

### Filter errors
Show only error-level log lines for an app.
```logql
{app="myapp"} |= "error"
```

### Count errors per minute
Rate of error log lines grouped by app.
```logql
sum by (app) (rate({namespace="production"} |= "error" [1m]))
```

### Parse JSON and filter field
Parse JSON logs and filter on a specific field value.
```logql
{app="myapp"} | json | status_code >= 500
```

## TraceQL

### Slow traces
Find traces where the root span duration exceeds 1 second.
```traceql
{ .http.status_code = 200 && duration > 1s }
```

### Traces with errors
Find all traces containing an error span.
```traceql
{ status = error }
```

### Traces by service
Find traces for a specific service with high latency.
```traceql
{ resource.service.name = "frontend" && duration > 500ms }
```
