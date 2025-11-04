package netutil

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// GetPublicIPv4 获取公网 IPv4 地址
func GetPublicIPv4() (string, error) {
	// 尝试多个服务，提高成功率
	services := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
		"https://ident.me",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, service := range services {
		ip, err := getIPFromService(ctx, service)
		if err == nil && ip != "" && isValidIPv4(ip) {
			return ip, nil
		}
	}

	// 如果所有外部服务都失败，尝试获取本地网络接口 IP
	return getLocalIPv4()
}

// GetPublicIPv6 获取公网 IPv6 地址
func GetPublicIPv6() (string, error) {
	// 尝试多个支持 IPv6 的服务
	services := []string{
		"https://api6.ipify.org",
		"https://ifconfig.co/ip",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, service := range services {
		ip, err := getIPFromService(ctx, service)
		if err == nil && ip != "" && isValidIPv6(ip) {
			return ip, nil
		}
	}

	// 如果所有外部服务都失败，尝试获取本地 IPv6 地址
	return getLocalIPv6()
}

// getIPFromService 从外部服务获取 IP
func getIPFromService(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	ip := strings.TrimSpace(string(body))
	return ip, nil
}

// getLocalIPv4 获取本地网络接口的 IPv4 地址
func getLocalIPv4() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}

	return "", nil
}

// getLocalIPv6 获取本地网络接口的 IPv6 地址
func getLocalIPv6() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() == nil && ipNet.IP.To16() != nil {
				// 过滤掉链路本地地址（fe80::）
				if !ipNet.IP.IsLinkLocalUnicast() {
					return ipNet.IP.String(), nil
				}
			}
		}
	}

	return "", nil
}

// isValidIPv4 验证是否为有效的 IPv4 地址
func isValidIPv4(ip string) bool {
	parsedIP := net.ParseIP(ip)
	return parsedIP != nil && parsedIP.To4() != nil
}

// isValidIPv6 验证是否为有效的 IPv6 地址
func isValidIPv6(ip string) bool {
	parsedIP := net.ParseIP(ip)
	return parsedIP != nil && parsedIP.To4() == nil && parsedIP.To16() != nil
}

// GetOutboundIP 通过连接外部服务获取出站 IP（用于 NAT 环境）
func GetOutboundIP() (string, error) {
	// 尝试连接 Google DNS，但不发送任何数据
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}
