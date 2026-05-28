# oops

CLI cheatsheet for PromQL, LogQL, and TraceQL queries. Ask a question in plain English and get back the most relevant query snippets from your team's markdown cheatsheet stored in Azure DevOps.

```
$ oops "p99 latency degradation"

2 result(s) for "p99 latency degradation"

[PromQL] P99 latency degradation vs 1h ago
Incident. Compares current P99 against the value from 1 hour ago; fires on 2x regression.

    histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))
      > 2 * histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m] offset 1h))

[PromQL] P99 latency by service
Day-to-day / SLO tracking. 99th-percentile latency — closer to worst-case user experience.

    histogram_quantile(0.99,
      sum(rate(http_request_duration_seconds_bucket[5m])) by (le, service)
    )
```

## Install

```bash
git clone https://github.com/pathcl/oops
cd oops
go build -o oops .
sudo mv oops /usr/local/bin/
```

## Configuration

Create `~/.config/oops/config.yaml`:

```yaml
azure_devops:
  org: my-org
  project: my-project
  repo: my-repo
  file_path: /docs/sre-cheatsheet.md
  branch: main      # optional, defaults to main
cache_ttl: 1h       # optional, defaults to 1h
```

All fields can also be set via environment variables:

| Variable              | Description                        |
|-----------------------|------------------------------------|
| `OOPS_ADO_ORG`        | Azure DevOps organisation name     |
| `OOPS_ADO_PROJECT`    | Azure DevOps project name          |
| `OOPS_ADO_REPO`       | Git repository name                |
| `OOPS_ADO_FILE_PATH`  | Path to the markdown file in repo  |
| `OOPS_ADO_BRANCH`     | Branch name (default: `main`)      |

## Authentication

`oops` uses the Azure CLI for authentication. Log in once before using the tool:

```bash
az login
```

A token is fetched automatically on each run (or cache refresh). No separate token management is needed.

## Usage

```bash
# Ask a question
oops "how do I calculate error rate?"
oops "slow database queries"
oops "pod crash loop"
oops "OOM killed memory"

# Force re-fetch the cheatsheet from Azure DevOps (ignore cache)
oops --refresh "p99 latency"

# Use a local markdown file instead of Azure DevOps (no auth required)
oops --file ./testdata/cheatsheet.md "connection timeout"
```

Results are printed to stdout and can be piped:

```bash
oops "error rate" | grep promql
```

## Cheatsheet format

The markdown file in Azure DevOps must follow this structure:

```markdown
## PromQL

### Rate of requests
Description of what this query does and when to use it.
```promql
rate(http_requests_total[5m])
```

## LogQL

### Filter errors
```logql
{app="myapp"} |= "error"
```
```

- `##` heading — query language / category (shown as the result badge)
- `###` heading — snippet title (searchable)
- Text between the heading and the code block — description (searchable)
- Fenced code block — the query itself

A starter cheatsheet covering PromQL, LogQL, and TraceQL across day-to-day, incident, and troubleshooting contexts is included at [`testdata/cheatsheet.md`](testdata/cheatsheet.md).

## Caching

The cheatsheet is cached at `~/.cache/oops/cheatsheet.md` with a default TTL of 1 hour. After the TTL expires the file is re-fetched from Azure DevOps automatically. Use `--refresh` to force an immediate update.

## Local testing

No Azure DevOps account needed for local testing:

```bash
oops --file testdata/cheatsheet.md "slow traces"
```
