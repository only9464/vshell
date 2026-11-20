package mcp

// MCP 资源：把仓库里的文档暴露成只读资源，AI 客户端可以直接读取项目状态，
// 不必让用户粘贴文件。
//
// URI 约定：vshell://docs/<文件名>（只允许 docs/ 目录下的白名单文件，
// 防止通过 MCP 读取任意路径）。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// docWhitelist 是允许通过 MCP 读取的文档（相对 docs/）。
var docWhitelist = []struct {
	File string
	Name string
	Desc string
}{
	{"STATUS.md", "还原状态", "各模块已实现/部分/缺失的当前判定（附证据）"},
	{"ROADMAP.md", "路线图", "里程碑、任务清单与验收标准"},
	{"PROTOCOL.md", "线协议", "传输帧、KCP/DNS/WS、记录与通道帧、文件/插件/隧道命令"},
	{"API.md", "面板 API", "HTTP/WS 端点契约与错误文案"},
	{"MCP.md", "MCP 说明", "本服务的工具清单与用法"},
	{"REFERENCE.md", "原版参考", "活体 4.9.3 的黑盒行为与工具链"},
}

// docResources 返回 resources/list 的结果。
func docResources() []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(docWhitelist))
	for _, d := range docWhitelist {
		out = append(out, map[string]interface{}{
			"uri":         "vshell://docs/" + d.File,
			"name":        d.Name,
			"description": d.Desc,
			"mimeType":    "text/markdown",
		})
	}
	return out
}

// docsRoot 找到仓库里的 docs 目录（从 cwd 向上最多找 4 层）。
//
// MCP 正常从仓库根目录启动；单元测试的 cwd 是包目录，因此需要向上找。
func docsRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 4; i++ {
		candidate := filepath.Join(dir, "docs")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("docs directory not found (run from the repository root)")
}

// readDocResource 读取白名单内的文档。
func readDocResource(uri string) (string, error) {
	const prefix = "vshell://docs/"
	if !strings.HasPrefix(uri, prefix) {
		return "", fmt.Errorf("unsupported resource uri: %s (want %s<file>)", uri, prefix)
	}
	name := strings.TrimPrefix(uri, prefix)
	for _, d := range docWhitelist {
		if d.File == name {
			root, err := docsRoot()
			if err != nil {
				return "", err
			}
			data, err := os.ReadFile(filepath.Join(root, d.File))
			if err != nil {
				return "", fmt.Errorf("read %s: %w", name, err)
			}
			return string(data), nil
		}
	}
	return "", fmt.Errorf("resource not in whitelist: %s", name)
}
