package c2engine

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

// 黄金向量：官方实例（4.9.3）对
//
//	/download/listen?arch=linux_amd64&tp=tcp&host=1.2.3.4&port=4444
//	                 &vkey=vk-probe&salt=salty&upx=false
//
// 的真实产物。区域偏移 0x4c6339、长度 20480，由二进制模板与产物逐字节对比得到。
const (
	// 黄金向量：从官方产物里切下来的 20480 字节配置区（见同目录 meta.json）。
	goldenPath = "../testdata/original/payload/config_region_tcp_linux_amd64.bin"
)

func loadGolden(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Skipf("golden payload missing: %v", err)
	}
	return data
}

// 复刻端的编码器必须与官方产物**逐字节一致**。
func TestAgentConfigMatchesOfficialBytes(t *testing.T) {
	want := loadGolden(t)
	if len(want) != AgentConfigRegionSize {
		t.Fatalf("golden region size = %d, want %d", len(want), AgentConfigRegionSize)
	}

	got, err := EncodeAgentConfigRegion(AgentWireConfig{
		Server: "1.2.3.4:4444",
		Type:   "tcp",
		Vkey:   "vk-probe",
		Proxy:  "",
		Salt:   "salty",
		L:      true,
		E:      false,
		D:      30,
		H:      10,
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !bytes.Equal(got, want) {
		// 找到第一个分歧字节，便于定位
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("config region differs at %#x: got %02x want %02x", i, got[i], want[i])
			}
		}
		t.Fatalf("config region differs (length %d vs %d)", len(got), len(want))
	}
}

// 解码官方区域，验证 JSON 字段结构。
func TestDecodeOfficialRegion(t *testing.T) {
	cfg, err := DecodeAgentConfigRegion(loadGolden(t), "vk-probe")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.Server != "1.2.3.4:4444" || cfg.Type != "tcp" || cfg.Vkey != "vk-probe" ||
		cfg.Salt != "salty" || !cfg.L || cfg.E || cfg.D != 30 || cfg.H != 10 {
		raw, _ := json.Marshal(cfg)
		t.Fatalf("decoded config = %s", raw)
	}
}

// 区域定位：在真实模板（含 0x19 占位区）里能找到，并落地改写。
func TestPatchAgentConfigInTemplate(t *testing.T) {
	// 造一个带占位区的模板
	tpl := make([]byte, AgentConfigRegionSize+128)
	for i := 0; i < len(tpl); i++ {
		tpl[i] = 0x90
	}
	for i := 0; i < AgentConfigRegionSize; i++ {
		tpl[64+i] = 0x19
	}
	off, err := PatchAgentConfig(tpl, AgentWireConfig{Server: "h:1", Type: "ws", Vkey: "k"})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if off != 64 {
		t.Fatalf("offset = %d, want 64", off)
	}
	// 写进去的内容必须能被解出来
	cfg, err := DecodeAgentConfigRegion(tpl[off:off+AgentConfigRegionSize], "k")
	if err != nil {
		t.Fatalf("decode patched: %v", err)
	}
	if cfg.Server != "h:1" || cfg.Type != "ws" {
		t.Fatalf("decoded = %+v", cfg)
	}
	// 模板其它字节不受影响
	if tpl[0] != 0x90 || tpl[len(tpl)-1] != 0x90 {
		t.Fatalf("template padding was modified")
	}
}

// 找不到占位区时必须明确报错，而不是静默追加。
func TestPatchAgentConfigWithoutRegion(t *testing.T) {
	tpl := bytes.Repeat([]byte{0x90}, 4096)
	if _, err := PatchAgentConfig(tpl, AgentWireConfig{Vkey: "k"}); err == nil {
		t.Fatalf("expected ErrNoConfigRegion")
	}
}

// 明文超长要报错（固定区域 20464 字节）。
func TestAgentConfigTooLarge(t *testing.T) {
	big := make([]byte, AgentConfigPlainSize)
	for i := range big {
		big[i] = 'a'
	}
	_, err := EncodeAgentConfigRegion(AgentWireConfig{Server: string(big), Vkey: "k"})
	if err == nil {
		t.Fatalf("expected size error")
	}
}

// PoBuild 生成的产物里，官方配置区必须可解且字段正确。
func TestPoBuildWritesOfficialConfigRegion(t *testing.T) {
	// 造一个"模板"：带 0x19 占位区的假二进制
	dir := t.TempDir()
	tpl := bytes.Repeat([]byte{0x7f}, 512)
	tpl = append(tpl, bytes.Repeat([]byte{0x19}, AgentConfigRegionSize)...)
	tpl = append(tpl, bytes.Repeat([]byte{0x7f}, 512)...)
	path := dir + "/stageless/http_windows_amd64"
	if err := os.MkdirAll(dir+"/stageless", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, tpl, 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	prev, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	l := &Listener{ID: 1, Mode: "http", ListenAddr: "127.0.0.1:8443", VerifyKey: "vk1", EncryptSalt: "s1"}
	p := PoBuild(l, PoConfig{Mode: "stageless", Arch: "windows_amd64"})
	if p.Err != nil {
		t.Fatalf("PoBuild: %v", p.Err)
	}
	cfg, err := DecodeAgentConfigRegion(p.Data[512:512+AgentConfigRegionSize], "vk1")
	if err != nil {
		t.Fatalf("decode region: %v", err)
	}
	if cfg.Vkey != "vk1" || cfg.Salt != "s1" || cfg.Type != "http" {
		t.Fatalf("config = %+v", cfg)
	}
	if cfg.Server != "http://127.0.0.1:8443" && cfg.Server != "127.0.0.1:8443" {
		t.Fatalf("server = %q", cfg.Server)
	}
}
