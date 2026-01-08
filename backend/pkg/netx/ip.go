package netx

import "net"

// GetOutboundIP 获得对外发送消息的 IP 地址
func GetOutboundIP() string {
	// DNS 的地址，国内可以用 114.114.114.114
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}
