package c2engine

// 官方 tcp 传输的**任务记录**结构（R21 MITM 实证）。
//
// 抓包得到的明文信封（解密后）：
//
//	[u32 LE 长度][type u8][channel u32 LE][payload]
//	type 0x06 = 服务端 ACK，0x03 = 数据，0x05 = 进度，0x07 = 结束，0x01 = 客户端 ACK
//
// 数据载荷（type=0x03）里的 payload 是 **JSON 任务**：
//
//	{"ConnType":"v","Host":"v","LocalProxy":false,"RemoteAddr":"v",
//	 "Req":{"Pass":"v","Type":"M","File":null,"Z1":"id","Z2":"","Z3":"","Z4":"","Z5":""},
//	 "Option":{"Timeout":5000000000}}
//
// Req.Type 是操作类型码，Z1..Z5 是位置参数（见 OfficialTaskType* 常量），
// File 是 base64 内容（上传时非空）。结果回传形如 `->|<输出>|<-`。
//
// 复刻端用途：作为**保真度基准**——可以把同样的操作编成官方格式用于对比/测试；
// 复刻端自身的 agent 协议保持不变（见 docs/GAPS.md C 类偏差）。

import (
	"encoding/base64"
	"encoding/json"
	"errors"
)

// 官方任务类型码（Req.Type）。
const (
	OfficialTaskExec    = "M"      // 执行命令：Z1 = 命令行
	OfficialTaskListDir = "B"      // 列目录：Z1 = 目录
	OfficialTaskCat     = "C"      // 读文件：Z1 = 完整路径
	OfficialTaskDisk    = "A"      // 磁盘信息：无参数
	OfficialTaskTouch   = "D"      // 新建/touch：Z1 = 路径
	OfficialTaskMkdir   = "J"      // 建目录：Z1 = 目录（末尾带 /）
	OfficialTaskMove    = "I"      // 移动/重命名：Z1 = Z2 = "src|<-dst/"
	OfficialTaskDelete  = "DSELF"  // 删除文件：无参数
	OfficialTaskUpload  = "Upload" // 上传：File = base64 内容，Z1 = 目标路径
)

// OfficialTaskReq 是任务记录的 Req 段。
type OfficialTaskReq struct {
	Pass string  `json:"Pass"`
	Type string  `json:"Type"`
	File *string `json:"File"` // nil → JSON null；上传时是 base64 字符串
	Z1   string  `json:"Z1"`
	Z2   string  `json:"Z2"`
	Z3   string  `json:"Z3"`
	Z4   string  `json:"Z4"`
	Z5   string  `json:"Z5"`
}

// OfficialTaskOption 是任务记录的可选项段。
type OfficialTaskOption struct {
	// Timeout 是纳秒；官方默认 5e9（5 秒）。
	Timeout int64 `json:"Timeout"`
}

// OfficialTask 是官方任务记录的完整结构（字段顺序与抓包一致）。
type OfficialTask struct {
	ConnType   string             `json:"ConnType"`
	Host       string             `json:"Host"`
	LocalProxy bool               `json:"LocalProxy"`
	RemoteAddr string             `json:"RemoteAddr"`
	Req        OfficialTaskReq    `json:"Req"`
	Option     OfficialTaskOption `json:"Option"`
}

// NewOfficialTask 构造一条官方格式任务（ConnType/Host/RemoteAddr/Pass 固定为 "v"，
// 与抓包一致）。
func NewOfficialTask(typ string, z ...string) *OfficialTask {
	t := &OfficialTask{
		ConnType:   "v",
		Host:       "v",
		RemoteAddr: "v",
		Req:        OfficialTaskReq{Pass: "v", Type: typ},
		Option:     OfficialTaskOption{Timeout: 5_000_000_000},
	}
	fields := []*string{&t.Req.Z1, &t.Req.Z2, &t.Req.Z3, &t.Req.Z4, &t.Req.Z5}
	for i, v := range z {
		if i < len(fields) {
			*fields[i] = v
		}
	}
	return t
}

// WithUploadFile 把上传内容写进 File 字段（base64）。
func (t *OfficialTask) WithUploadFile(content []byte) *OfficialTask {
	enc := base64.StdEncoding.EncodeToString(content)
	t.Req.File = &enc
	return t
}

// Encode 序列化成官方线格式的 JSON（字段顺序固定）。
func (t *OfficialTask) Encode() ([]byte, error) { return json.Marshal(t) }

// DecodeOfficialTask 解析官方任务记录。
func DecodeOfficialTask(data []byte) (*OfficialTask, error) {
	var t OfficialTask
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	if t.Req.Type == "" {
		return nil, errors.New("missing Req.Type")
	}
	return &t, nil
}

// Command 返回可执行的命令字符串（仅对 M 类型有意义）。
func (t *OfficialTask) Command() string {
	if t == nil || t.Req.Type != OfficialTaskExec {
		return ""
	}
	return t.Req.Z1
}
