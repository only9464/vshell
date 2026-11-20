// Package controllers — 客户端在线巡检。
//
// 原版维护客户端在线位（结构 +0x50）的常驻后台对象来自 FUN_0194d140，其类型
// 与运行循环未能从 garble 二进制恢复。这里用最小可信实现补上这一环：
// 各传输通道在每次交互时都会刷新 LastSeen（HTTP 任务轮询、KCP 结果/心跳、
// DNS 查询），因此"静默超时"即可判定离线。
//
// 没有这一步时 Status 永远为假，面板的终端/文件/截图/插件等全部下发接口都会
// 返回 "client is close"（见 docs/STATUS.md §4-③）。
package controllers

import (
	"log"
	"sync"
	"time"

	"vshell/c2engine"
)

const (
	// clientOnlineSweepInterval 是在线巡检周期。
	clientOnlineSweepInterval = 15 * time.Second
	// clientOnlineTimeout 是判定离线的静默时长，与监听器 DisconnectTimeout
	// 的默认值（60s）保持一致。
	clientOnlineTimeout = 60 * time.Second
)

var clientSweeperOnce sync.Once

// startClientOnlineSweeper 启动在线巡检 goroutine（幂等）。
// startClientOnlineSweeper starts the liveness sweeper (idempotent).
func startClientOnlineSweeper() {
	clientSweeperOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(clientOnlineSweepInterval)
			defer ticker.Stop()
			for range ticker.C {
				if n := c2engine.GetEngine().SweepOffline(clientOnlineTimeout); n > 0 {
					log.Printf("[C2Engine] liveness sweep: %d client(s) marked offline", n)
				}
			}
		}()
	})
}
