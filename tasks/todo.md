# oops - SRE Query Cheatsheet CLI

## Tasks

- [x] Project scaffold + go.mod
- [ ] internal/config — YAML config + env var overrides
- [ ] internal/adoclient — az CLI token + ADO REST fetch
- [ ] internal/cache — local file cache with TTL
- [ ] internal/parser — goldmark markdown → []Section
- [ ] internal/search — BM25 scorer
- [ ] internal/tui — bubbletea list + detail view
- [ ] cmd/root.go — cobra root command wiring
- [ ] main.go — entry point
- [ ] Tests: parser, BM25, cache TTL
- [ ] Manual end-to-end verification
