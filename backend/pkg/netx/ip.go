package netx

import "net"

// GetOutboundIP 鑾峰緱瀵瑰鍙戦€佹秷鎭殑 IP 鍦板潃
func GetOutboundIP() string {
	// DNS 鐨勫湴鍧€锛屽浗鍐呭彲浠ョ敤 114.114.114.114
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}


