package c2engine

import (
	"os"
	"path/filepath"
	"testing"
)

// 配置块：写入 + 从文件尾部读回（agent 自解析路径）。
func TestAgentConfigBlockRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.bin")

	template := []byte("MZ-fake-template-bytes")
	cfg := map[string]string{
		"server": "http://127.0.0.1:18080",
		"vkey":   "vk_demo",
		"salt":   "salt_demo",
		"mode":   "http",
	}
	out, err := AppendAgentConfigBlock(template, cfg)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if len(out) <= len(template) {
		t.Fatalf("payload should be larger than template")
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := ReadAgentConfigBlockFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	for k, v := range cfg {
		if got[k] != v {
			t.Fatalf("field %q = %q, want %q", k, got[k], v)
		}
	}
	// 模板前缀必须逐字节保留。
	if string(out[:len(template)]) != string(template) {
		t.Fatalf("template prefix was modified")
	}
}

// 没有配置块的文件必须返回 ErrNoAgentConfig（agent 据此回落到 ldflags）。
func TestAgentConfigBlockAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.bin")
	if err := os.WriteFile(path, []byte("just a template, no trailer at all"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := ReadAgentConfigBlockFile(path); err != ErrNoAgentConfig {
		t.Fatalf("err = %v, want ErrNoAgentConfig", err)
	}
}

// 尾部魔数对但长度非法时也必须拒绝（防越界）。
func TestAgentConfigBlockRejectsBadLength(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.bin")
	body := append([]byte("junk"), 0xff, 0xff, 0xff, 0xff)
	body = append(body, []byte(AgentConfigMagic)...)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := ReadAgentConfigBlockFile(path); err != ErrNoAgentConfig {
		t.Fatalf("err = %v, want ErrNoAgentConfig", err)
	}
}
