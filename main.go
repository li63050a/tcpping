package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"sort"
	"time"
)

// resolveHost 解析主机名到 IP 地址
// 优先使用 Go 内置解析，失败时调用系统 ping 命令获取 IP
func resolveHost(host string) (string, error) {
	// 如果已经是 IP 地址，直接返回
	if ip := net.ParseIP(host); ip != nil {
		return host, nil
	}

	// 尝试 Go 内置 DNS 解析
	ips, err := net.LookupHost(host)
	if err == nil && len(ips) > 0 {
		return ips[0], nil
	}

	// 内置解析失败，使用 ping 命令解析
	fmt.Printf("内置 DNS 解析失败，尝试用 ping 解析 %s ...\n", host)
	return resolveViaPing(host)
}

// resolveViaPing 通过 ping 命令提取域名对应的 IP
func resolveViaPing(host string) (string, error) {
	out, err := exec.Command("ping", "-c", "1", host).Output()
	if err != nil {
		return "", fmt.Errorf("ping 命令执行失败: %v", err)
	}

	output := string(out)

	// 匹配 "PING baidu.com (124.237.177.164)" 格式
	re := regexp.MustCompile(`PING\s+\S+\s+\(([0-9.]+)\)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		return matches[1], nil
	}

	// 如果上面没匹配，可能 ping 直接给了 IP（如 "PING 124.237.177.164"）
	re2 := regexp.MustCompile(`PING\s+([0-9.]+)`)
	matches2 := re2.FindStringSubmatch(output)
	if len(matches2) > 1 {
		return matches2[1], nil
	}

	return "", fmt.Errorf("无法从 ping 输出中提取 IP:\n%s", output)
}

func main() {
	count := flag.Int("c", 0, "发送次数（0 表示无限）")
	interval := flag.Duration("i", time.Second, "间隔时间（如 1s, 500ms）")
	timeout := flag.Duration("t", time.Second, "连接超时（如 2s）")
	flag.Parse()

	args := flag.Args()
	if len(args) < 2 {
		fmt.Println("用法: tcpping <目标地址> <端口> [-c 次数] [-i 间隔] [-t 超时]")
		fmt.Println("示例: tcpping 10.0.0.1 80 -c 4")
		os.Exit(1)
	}
	host := args[0]
	port := args[1]

	// 检查端口是否是数字
	if _, err := fmt.Sscanf(port, "%d", new(int)); err != nil {
		fmt.Printf("错误：端口必须为数字，得到 %q\n", port)
		os.Exit(1)
	}

	// 解析主机
	ip, err := resolveHost(host)
	if err != nil {
		fmt.Printf("解析主机 %s 失败: %v\n", host, err)
		os.Exit(1)
	}
	if ip != host {
		fmt.Printf("解析 %s → %s\n", host, ip)
	}

	target := net.JoinHostPort(ip, port)
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