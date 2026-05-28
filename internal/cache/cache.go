package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type meta struct {
	FetchedAt time.Time `json:"fetched_at"`
}

// Cache manages a local copy of the cheatsheet markdown with TTL expiry.
type Cache struct {
	dir string
	ttl time.Duration
}

func New(ttl time.Duration) *Cache {
	return &Cache{
		dir: cacheDir(),
		ttl: ttl,
	}
}

func (c *Cache) IsStale() bool {
	m, err := c.readMeta()
	if err != nil {
		return true
	}
	return time.Since(m.FetchedAt) > c.ttl
}

func (c *Cache) Read() (string, error) {
	data, err := os.ReadFile(c.markdownPath())
	if err != nil {
		return "", fmt.Errorf("reading cache: %w", err)
	}
	return string(data), nil
}

func (c *Cache) Write(content string) error {
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}
	if err := os.WriteFile(c.markdownPath(), []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing cache: %w", err)
	}
	return c.writeMeta(meta{FetchedAt: time.Now()})
}

func (c *Cache) markdownPath() string {
	return filepath.Join(c.dir, "cheatsheet.md")
}

func (c *Cache) metaPath() string {
	return filepath.Join(c.dir, "meta.json")
}

func (c *Cache) readMeta() (meta, error) {
	data, err := os.ReadFile(c.metaPath())
	if err != nil {
		return meta{}, err
	}
	var m meta
	if err := json.Unmarshal(data, &m); err != nil {
		return meta{}, err
	}
	return m, nil
}

func (c *Cache) writeMeta(m meta) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(c.metaPath(), data, 0o600)
}

func cacheDir() string {
	if d := os.Getenv("XDG_CACHE_HOME"); d != "" {
		return filepath.Join(d, "oops")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "oops")
}
