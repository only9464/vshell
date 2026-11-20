package c2engine

import "os"

// osReadDir 返回目录项名称（模拟器用，避免在主体文件里引入 os 依赖噪声）。
func osReadDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out, nil
}
