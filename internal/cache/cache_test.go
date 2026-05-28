package cache

import (
	"os"
	"testing"
	"time"
)

func newTestCache(t *testing.T, ttl time.Duration) *Cache {
	t.Helper()
	dir := t.TempDir()
	return &Cache{dir: dir, ttl: ttl}
}

func TestCache_WriteAndRead(t *testing.T) {
	c := newTestCache(t, time.Hour)
	content := "## PromQL\n### Rate\n```promql\nrate(x[5m])\n```"
	if err := c.Write(content); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := c.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != content {
		t.Errorf("Read returned %q, want %q", got, content)
	}
}

func TestCache_IsStale_Fresh(t *testing.T) {
	c := newTestCache(t, time.Hour)
	if err := c.Write("content"); err != nil {
		t.Fatal(err)
	}
	if c.IsStale() {
		t.Error("expected cache to be fresh immediately after write")
	}
}

func TestCache_IsStale_Expired(t *testing.T) {
	c := newTestCache(t, time.Millisecond)
	if err := c.Write("content"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if !c.IsStale() {
		t.Error("expected cache to be stale after TTL expiry")
	}
}

func TestCache_IsStale_NoCache(t *testing.T) {
	c := newTestCache(t, time.Hour)
	if !c.IsStale() {
		t.Error("expected cache to be stale when no cache file exists")
	}
}

func TestCache_Read_MissingFile(t *testing.T) {
	c := newTestCache(t, time.Hour)
	_, err := c.Read()
	if !os.IsNotExist(err) {
		// Read wraps the error, so just check it's non-nil
		if err == nil {
			t.Error("expected error reading missing cache file")
		}
	}
}
