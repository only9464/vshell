// 示例插件：在目标机上打印主机信息。面板「插件」页选中后执行，输出会回显。
package main

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
	"time"
)

func main() {
	host, _ := os.Hostname()
	u, _ := user.Current()
	fmt.Printf("host=%s os=%s/%s\n", host, runtime.GOOS, runtime.GOARCH)
	if u != nil {
		fmt.Printf("user=%s uid=%s\n", u.Username, u.Uid)
	}
	fmt.Printf("pid=%d time=%s\n", os.Getpid(), time.Now().Format(time.RFC3339))
	if len(os.Args) > 1 {
		fmt.Printf("args=%v\n", os.Args[1:])
	}
}
