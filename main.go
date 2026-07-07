package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sort"
	"time"
)

func main() {
	count := flag.Int("c", 0, "发送次数（0 表示无限）")
	interval := flag.Duration("i", time.Second, "间隔时间（如 1s, 500ms）")
	timeout := flag.Duration("t", time.Second, "连接超时（如 2s）")
	flag.Parse()

	// 取位置参数：第一个是主机，第二个是端口
	args := flag.Args()
	if len(args) < 2 {
		fmt.Println("用法: tcpping <目标地址> <端口> [-c 次数] [-i 间隔] [-t 超时]")
		fmt.Println("示例: tcpping 10.0.0.1 80 -c 4")
		os.Exit(1)
	}
	host := args[0]
	port := args[1] // 字符串形式，下面会检查是否是有效数字

	// 简单检查端口是否为纯数字
	if _, err := fmt.Sscanf(port, "%d", new(int)); err != nil {
		fmt.Printf("错误：端口必须为数字，得到 %q\n", port)
		os.Exit(1)
	}

	target := net.JoinHostPort(host, port)
	fmt.Printf("TCP Ping %s (端口 %s)\n", host, port)

	var (
		sent, received int
		delays         []float64
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	done := false
	for !done {
		select {
		case <-sigCh:
			done = true
			continue
		case <-ticker.C:
			if *count > 0 && sent >= *count {
				done = true
				continue
			}

			sent++
			start := time.Now()
			conn, err := net.DialTimeout("tcp", target, *timeout)
			elapsed := time.Since(start).Seconds() * 1000

			if err != nil {
				fmt.Printf("连接 %s 失败: %v (%.2f ms)\n", target, err, elapsed)
			} else {
				conn.Close()
				received++
				delays = append(delays, elapsed)
				fmt.Printf("来自 %s 的应答: 时间=%.2f ms\n", target, elapsed)
			}
		}
	}

	// 统计
	fmt.Println("\n--- TCP Ping 统计 ---")
	if sent == 0 {
		fmt.Println("未发送任何包。")
		return
	}
	fmt.Printf("%d 个包已发送，%d 个包已接收，%.1f%% 丢包\n",
		sent, received, float64(sent-received)/float64(sent)*100)
	if len(delays) > 0 {
		sort.Float64s(delays)
		min := delays[0]
		max := delays[len(delays)-1]
		sum := 0.0
		for _, d := range delays {
			sum += d
		}
		avg := sum / float64(len(delays))
		fmt.Printf("最小/平均/最大延迟 = %.2f/%.2f/%.2f ms\n", min, avg, max)
	}
}