// Package mcp 提供后端原生的 MCP（Model Context Protocol）服务。
//
// 目标：让 AI 客户端（Claude Desktop、dsh 的 MCP connector 等）通过标准 MCP
// 直接操作这台 C2：列客户端、下发命令、传文件、开隧道、取截图……
// 不必再经过 HTTP 面板，也不依赖前端。
//
// 传输：stdio（JSON-RPC 2.0，每行一条消息），由 `vshell-server mcp` 启动。
// 之所以先做 stdio：MCP 客户端普遍支持，进程内直连引擎、无需额外端口与鉴权。
//
// 已实现的 MCP 方法：
//
//	initialize                  → 协议版本与工具能力
//	tools/list                  → 工具清单（含 JSON Schema）
//	tools/call                  → 调用具体工具
//	ping                        → 健康检查
//
// 工具清单见 tools.go；每个工具都直接调用 controllers/c2engine 的现有实现，
// 与面板走同一条业务路径（避免出现"面板能做的事 MCP 做不了"）。
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
)

// ProtocolVersion 是实现的 MCP 协议版本。
const ProtocolVersion = "2024-11-05"

// Server 是 stdio MCP 服务。
type Server struct {
	mu    sync.Mutex
	out   *bufio.Writer
	tools []Tool
}

// NewServer 创建服务并注册工具。
func NewServer(out io.Writer) *Server {
	s := &Server{
		out:   bufio.NewWriter(out),
		tools: builtinTools(),
	}
	return s
}

// request 是 JSON-RPC 2.0 请求（也是响应的形状）。
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve 读 stdin 上的 JSON-RPC 请求并逐条响应，直到 EOF。
func (s *Server) Serve(in io.Reader) error {
	reader := bufio.NewReaderSize(in, 1<<20)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			s.handleLine(line)
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func (s *Server) handleLine(line []byte) {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		s.writeError(nil, -32700, "parse error: "+err.Error())
		return
	}
	// 通知（无 id）不需要响应。
	if len(req.ID) == 0 && req.Method != "" {
		return
	}

	switch req.Method {
	case "initialize":
		s.writeResult(req.ID, map[string]interface{}{
			"protocolVersion": ProtocolVersion,
			"capabilities": map[string]interface{}{
				"tools":     map[string]interface{}{"listChanged": false},
				"resources": map[string]interface{}{"subscribe": false, "listChanged": false},
			},
			"serverInfo": map[string]interface{}{
				"name":    "vshell",
				"version": "4.9.3-replica",
			},
		})
	case "ping":
		s.writeResult(req.ID, map[string]interface{}{})
	case "tools/list":
		s.writeResult(req.ID, map[string]interface{}{"tools": s.tools})
	case "resources/list":
		s.writeResult(req.ID, map[string]interface{}{"resources": docResources()})
	case "resources/read":
		var p struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			s.writeError(req.ID, -32602, "invalid params: "+err.Error())
			return
		}
		text, err := readDocResource(p.URI)
		if err != nil {
			s.writeError(req.ID, -32602, err.Error())
			return
		}
		s.writeResult(req.ID, map[string]interface{}{
			"contents": []map[string]interface{}{{
				"uri":      p.URI,
				"mimeType": "text/markdown",
				"text":     text,
			}},
		})
	case "tools/call":
		var p struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			s.writeError(req.ID, -32602, "invalid params: "+err.Error())
			return
		}
		text, isErr := s.callTool(p.Name, p.Arguments)
		result := map[string]interface{}{
			"content": []map[string]interface{}{{"type": "text", "text": text}},
		}
		if isErr {
			result["isError"] = true
		}
		s.writeResult(req.ID, result)
	default:
		s.writeError(req.ID, -32601, "method not found: "+req.Method)
	}
}

func (s *Server) writeResult(id json.RawMessage, result interface{}) {
	resp := request{JSONRPC: "2.0", ID: id, Result: result}
	s.write(resp)
}

func (s *Server) writeError(id json.RawMessage, code int, msg string) {
	resp := request{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
	s.write(resp)
}

func (s *Server) write(resp request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("[mcp] marshal response: %v", err)
		return
	}
	if _, err := s.out.Write(append(data, '\n')); err != nil {
		log.Printf("[mcp] write response: %v", err)
		return
	}
	_ = s.out.Flush()
}

// textResult 把任意值序列化成 MCP 的文本内容。
func textResult(v interface{}) (string, bool) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("marshal error: %v", err), true
	}
	return string(data), false
}
