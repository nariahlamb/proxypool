package healthcheck

import (
	"net"
	"strconv"
	"time"
)

// TCPCheck 轻量 TCP 连通性探测（冻结节点预检用）：
// 仅验证 ip:port 端口可达，成本远低于完整 URLTest（握手 + 数据往返）。
// 返回 true 表示可建立 TCP 连接。
func TCPCheck(host string, port int, timeout time.Duration) bool {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
