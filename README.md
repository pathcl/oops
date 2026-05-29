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
go install github.com/pathcl/oops@latest
```

The binary is placed in `$GOPATH/bin` (usually `~/go/bin`). Make sure that directory is on your `$PATH`.

**From source** (for development or when no tag is published yet):

```bash
git clone https://github.com/pathcl/oops
cd oops
go install .
```

## Configuration

Create `~/.config/oops/config.yaml` with **either** a `github` block or an `azure_devops` block:

**GitHub:**
```yaml
github:
  owner: my-org
  repo: sre-runbooks
  file_path: docs/cheatsheet.md
  branch: main      # optional, defaults to main
cache_ttl: 1h
```

**Azure DevOps:**
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

**GitHub:**

| Variable                | Description                          |
|-------------------------|--------------------------------------|
| `OOPS_GITHUB_OWNER`     | GitHub organisation or user name     |
| `OOPS_GITHUB_REPO`      | Repository name                      |
| `OOPS_GITHUB_FILE_PATH` | Path to the markdown file in repo    |
| `OOPS_GITHUB_BRANCH`    | Branch name (default: `main`)        |

**Azure DevOps:**

| Variable              | Description                        |
|-----------------------|------------------------------------|
| `OOPS_ADO_ORG`        | Azure DevOps organisation name     |
| `OOPS_ADO_PROJECT`    | Azure DevOps project name          |
| `OOPS_ADO_REPO`       | Git repository name                |
| `OOPS_ADO_FILE_PATH`  | Path to the markdown file in repo  |
| `OOPS_ADO_BRANCH`     | Branch name (default: `main`)      |

## Authentication

`oops` uses the CLI tool for whichever provider you configure. Log in once before using the tool:

**GitHub** — requires the [gh CLI](https://cli.github.com):
```bash
gh auth login
```

**Azure DevOps** — requires the [az CLI](https://learn.microsoft.com/cli/azure):
```bash
az login
```

A token is fetched automatically on each run (or cache refresh). No separate token management is needed.

## Usage

Try it immediately with no configuration — the bundled cheatsheet works out of the box:

```
$ oops --file ./testdata/cheatsheet.md "slow traces threshold"

5 result(s) for "slow traces threshold"

[TraceQL] Traces slower than threshold
Incident. Traces where total end-to-end duration exceeds 5 seconds — active slowness investigation.

    { traceDuration > 5s }

[TraceQL] Slow traces
Day-to-day / SLO. Root spans exceeding 1 second — user-facing latency violations.

    { rootName = "GET /api/v1" && duration > 1s }

[TraceQL] Timeout exceptions across services
Troubleshooting. Spans that recorded a TimeoutError exception — isolates flaky dependencies.

    { event.exception.type = "TimeoutError" }

[TraceQL] Slow database queries
Troubleshooting. Database spans slower than 500ms — pinpoints query-level bottlenecks.

    { span.db.system = "postgresql" && duration > 500ms }

[TraceQL] Latency from a service to its dependencies
Troubleshooting. Client-side spans from one service — measures outbound latency to each dependency.

    { resource.service.name = "api" && span.kind = "client" }
```

```bash
# PromQL — metrics and alerting
oops "p99 latency degradation"
oops "5xx error rate spike"
oops "pod restarts crash loop"
oops "memory saturation container limit"
oops "top pods by cpu"

# LogQL — log filtering and aggregation
oops "fatal errors stack trace logs"
oops "connection refused timeout log"
oops "auth failures unauthorized logs"
oops "OOMKilled events"

# TraceQL — distributed tracing
oops "slow traces threshold 5 seconds"
oops "error spans payment endpoint"
oops "slow database queries postgres"
oops "timeout exceptions across services"

# Force re-fetch the cheatsheet from Azure DevOps (ignore cache)
oops --refresh "p99 latency degradation"

# Use a local markdown file instead of Azure DevOps (no auth required)
oops --file ./testdata/cheatsheet.md "slow traces threshold"

# Enable LLM reranking (Anthropic is the default provider)
export ANTHROPIC_API_KEY=sk-ant-...
oops --llm "error logs"
oops --llm --file ./testdata/cheatsheet.md "slow database queries"
```

Results are printed to stdout and can be piped:

```bash
oops "error rate logs" | grep LogQL
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

A reference can be found in our tests [testdata/cheatsheet.md](testdata/cheatsheet.md).

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

## LLM reranking

The `--llm` flag enables a hybrid search mode: BM25 retrieves the top 10 candidates and an LLM reranks them by semantic relevance before returning the final top 5. This improves results for vague or synonym-heavy queries where BM25 token matching falls short.

**Anthropic (default)**

```bash
export ANTHROPIC_API_KEY=sk-ant-...
oops --llm "error logs"
```

`claude-haiku-4-5` is used by default — fast and cheap for a reranking task. Override via config or env:

```yaml
# ~/.config/oops/config.yaml
llm:
  provider: anthropic
  model: claude-haiku-4-5   # or any claude-* model
  api_key: sk-ant-...       # optional, falls back to ANTHROPIC_API_KEY
```

**OpenAI**

```bash
export OOPS_LLM_PROVIDER=openai
export OPENAI_API_KEY=sk-...
oops --llm "slow database queries"
```

`gpt-4o-mini` is the default OpenAI model. Override with `OOPS_LLM_MODEL` or `llm.model` in config.

**Fallback behaviour**

If the LLM call fails (network error, missing API key, rate limit), `oops` prints a warning to stderr and falls back to the BM25 ranking — the tool always returns results.

## Local testing

No Azure DevOps account needed for local testing:

```bash
oops --file testdata/cheatsheet.md "slow traces"
oops --llm --file testdata/cheatsheet.md "slow traces"   # with LLM reranking
```

## How it works

```
┌─────────────────────────────────────────────────────────────────────┐
│                         oops "error logs"                           │
└───────────────┬─────────────────────────────┬───────────────────────┘
                │                             │ --file ./cheatsheet.md
                │                             ▼
                │              ┌──────────────────────────┐
                │              │   read local file        │
                │              │   os.ReadFile(path)      │
                │              └──────────────┬───────────┘
                │                             │
                ▼                             │
┌──────────────────────────┐                 │
│      Load config         │                 │
│  ~/.config/oops/         │                 │
│  config.yaml             │                 │
│  + env var overrides     │                 │
└──────────────┬───────────┘                 │
               │                             │
┌──────────────▼───────────┐                 │
│      Cache check         │──── hit ──┐     │
│  meta.json + TTL         │           │     │
└──────────────┬───────────┘           │     │
             miss                      │     │
┌──────────────▼───────────┐           │     │
│  az account              │           │     │
│  get-access-token        │           │     │
└──────────────┬───────────┘           │     │
               │ Bearer token          │     │
┌──────────────▼───────────┐           │     │
│  Azure DevOps REST API   │           │     │
│  GET /git/repositories/  │           │     │
│  {repo}/items?path=...   │           │     │
└──────────────┬───────────┘           │     │
               │ raw markdown          │     │
               ├── write cache ──▶ ~/.cache/ │
               │   (best-effort,    oops/    │
               │    warn on fail) cheatsheet │
               │                    .md ─────┘
               │◀──────────────────────────┘
               │ markdown
┌──────────────▼──────────────────────────────────────┐
│                   Parse markdown                     │
│                                                      │
│   ## Category  (H2) ─▶ Section.Category             │
│   ### Title    (H3) ─▶ Section.Title                │
│   description       ─▶ Section.Body                 │
│   ```lang  block    ─▶ Section.CodeBlock + Lang      │
└──────────────┬──────────────────────────────────────┘
               │ []Section
┌──────────────▼──────────────────────────────────────┐
│                  Search pipeline                     │
│                                                      │
│   "error logs"                                       │
│       │                                              │
│       ▼                                              │
│   stop word removal ── show, find, which, are...     │
│       │                                              │
│       ▼                                              │
│   Porter stemming ───── errors→error  logs→log       │
│       │                                              │
│       ▼                                              │
│   synonym expansion ─── log → logql, loki, event    │
│       │                 error → failure, fault       │
│       ▼                                              │
│   category detect ───── log → boost LogQL ×2.5      │
│       │                                              │
│       ▼                                              │
│   BM25 score ─────────── IDF × TF-sat × len-norm    │
│       │                  + category boost applied    │
│       ▼                                              │
│   threshold filter ───── drop < 30% of top score    │
│       │                                              │
│       ▼                                              │
│   stable sort ────────── score desc, title asc      │
│       │                                              │
│       ▼                                              │
│   top 5 results                                      │
└──────────────┬──────────────────────────────────────┘
               │
┌──────────────▼──────────────────────────────────────┐
│                      stdout                          │
│                                                      │
│   [LogQL] Error log rate per service                 │
│   Day-to-day. Rate of error-level lines...           │
│                                                      │
│       sum by (service) (rate({namespace=...          │
└──────────────────────────────────────────────────────┘
```

## Why?

### Why BM25 and not an LLM or vector search

The honest answer is that for this problem — a curated corpus of 30–300 snippets, queried by engineers who roughly know what they are looking for — BM25 outperforms more sophisticated approaches on every axis that matters in production: latency, reliability, cost, and trust in the output.

**BM25 is the proven baseline.** Introduced by Robertson et al. at TREC-3 in 1994 ([*Okapi at TREC-3*, Robertson et al., 1994](https://trec.nist.gov/pubs/trec3/papers/city.ps.gz)) and formalised in [*The Probabilistic Relevance Framework: BM25 and Beyond* (Robertson & Zaragoza, 2009)](https://www.nowpublishers.com/article/Details/INR-019), it has remained competitive against neural approaches for over three decades. A 2021 study by Thakur et al., [*BEIR: A Heterogeneous Benchmark for Zero-shot Evaluation of Information Retrieval Models*](https://arxiv.org/abs/2104.08663), benchmarked BM25 against dense neural retrievers across 18 datasets and found BM25 matched or outperformed neural models on 7 of them — particularly on domain-specific corpora where the vocabulary is controlled. An SRE cheatsheet is exactly that kind of corpus.

**BM25 is what production search engines use.** Apache Lucene replaced TF-IDF with BM25 as its default similarity in version 6.0 (2016). Elasticsearch adopted it in version 5.0 the same year. Typesense, MeiliSearch, and Tantivy all ship BM25 as their default ranking function. When you search GitHub code search, Elasticsearch with BM25 is doing the ranking. It is not an academic choice — it is the algorithm behind the tools the industry trusts at scale.

**Neural search adds complexity without proportional benefit at this scale.** Dense retrieval models like OpenAI `text-embedding-3-small` or the open-source `bge-m3` produce semantically rich vectors, but require a network call (or a local model consuming 100MB–4GB of RAM), a vector store, and introduce latency in the 200ms–2s range. For a corpus under 500 documents, [*Pretrained Transformers for Text Ranking: BERT and Beyond* (Lin et al., 2021)](https://arxiv.org/abs/2010.06467) notes that the gains over BM25 on small, domain-specific collections are marginal and often within noise. The infrastructure cost is not justified.

**LLMs hallucinate. BM25 does not.** A generative model asked to return a PromQL query may produce something that looks correct but references metric names that do not exist in your Prometheus, uses deprecated label syntax, or subtly misremembers a histogram function. [*Survey of Hallucination in Natural Language Generation* (Ji et al., 2023)](https://dl.acm.org/doi/10.1145/3571730) documents this class of failure extensively. `oops` returns verbatim text from your cheatsheet — if it is in the output, it is exactly what your team wrote and tested.

### Why Porter stemming

The [Porter stemmer](https://tartarus.org/martin/PorterStemmer/) (Porter, 1980, *Program: Electronic Library and Information Systems*) reduces inflected word forms to a common root so that `"restarts"`, `"restarted"`, and `"restarting"` all match a section about `"restart"`. It is implemented here via the [Snowball](https://snowballstem.org/) framework, the same underlying engine used by Elasticsearch's `english` analyzer and PostgreSQL's full-text search. It is lightweight (no model, no dictionary lookup) and well-understood, with documented failure modes.

### Why a hand-curated synonym map

General-purpose synonyms (WordNet, ConceptNet) do not know that `pod` and `container` are the same thing in Kubernetes, or that `postgres` is a `database`. Domain synonym expansion is a standard technique in enterprise search — Elasticsearch exposes it through its [synonym token filter](https://www.elastic.co/guide/en/elasticsearch/reference/current/analysis-synonym-tokenfilter.html) for exactly this reason. Our map is small, auditable, and wrong in predictable ways, which is preferable to a black-box embedding that is wrong in unpredictable ways.

### Why not Google

Google returns results from the public internet. Your cheatsheet contains queries written against your metric names, your label selectors, your namespaces, your service topology. No public search engine has that. Beyond specificity, there are three operational reasons to prefer a local tool during an incident:

- **Availability.** VPN issues, DNS failures, or the incident itself may leave you without internet access. `oops` works offline against its local cache.
- **Speed.** A terminal query returning in under 5ms is faster than opening a browser, waiting for a results page, reading five articles, and copy-pasting from a code block.
- **Tribal knowledge.** The most valuable entries in your cheatsheet are the ones nobody else would publish: the query that caught last quarter's memory leak, the TraceQL snippet that isolates the payment provider timeout, the LogQL expression that surfaces the single log line preceding a cascade. Google does not know your system. Your cheatsheet does.
