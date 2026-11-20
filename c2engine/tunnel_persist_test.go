package c2engine

import (
	"path/filepath"
	"testing"
)

// 隧道持久化：增删后写 JSON 侧车，重启（新引擎 Init）能读回。
func TestTunnelPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{DBPath: filepath.Join(dir, "data.db")}

	e := &Engine{
		config:      cfg,
		clients:     make(map[int64]*Client),
		listeners:   make(map[int64]*Listener),
		tunnels:     make(map[int64]*Tunnel),
		hosts:       make(map[int64]*Host),
		tasks:       make(map[int64]*Task),
		vkeyIndex:   make(map[string]int64),
		blockedKeys: make(map[string]bool),
		storage:     NewStorage(cfg.DBPath),
	}
	defer e.storage.Close()

	tn, err := e.NewTunnel(0, 18080, "tcp", "127.0.0.1:22")
	if err != nil {
		t.Fatalf("new tunnel: %v", err)
	}
	tn.Remark = "persist-me"
	e.SaveTunnels()

	// 新引擎（模拟重启）加载侧车
	e2 := &Engine{
		config:    cfg,
		tunnels:   make(map[int64]*Tunnel),
		storage:   NewStorage(cfg.DBPath),
		clients:   make(map[int64]*Client),
		listeners: make(map[int64]*Listener),
		hosts:     make(map[int64]*Host),
		tasks:     make(map[int64]*Task),
	}
	defer e2.storage.Close()
	var seq int64
	if err := e2.storage.LoadTunnels(e2.tunnels, &seq, tunnelsPath(cfg)); err != nil {
		t.Fatalf("load: %v", err)
	}
	got, ok := e2.tunnels[tn.ID]
	if !ok {
		t.Fatalf("tunnel %d not restored (got %d tunnels)", tn.ID, len(e2.tunnels))
	}
	if got.Port != 18080 || got.TargetAddr != "127.0.0.1:22" || got.Mode != "tcp" {
		t.Fatalf("restored tunnel = %+v", got)
	}
	if seq != tn.ID {
		t.Fatalf("seq = %d, want %d", seq, tn.ID)
	}

	// 删除后再落盘，侧车里不应再有它
	if err := e.DelTunnel(tn.ID); err != nil {
		t.Fatalf("del: %v", err)
	}
	e3 := &Engine{config: cfg, tunnels: make(map[int64]*Tunnel), storage: NewStorage(cfg.DBPath)}
	defer e3.storage.Close()
	var seq3 int64
	_ = e3.storage.LoadTunnels(e3.tunnels, &seq3, tunnelsPath(cfg))
	if len(e3.tunnels) != 0 {
		t.Fatalf("tunnel should be gone after delete, got %d", len(e3.tunnels))
	}
}
