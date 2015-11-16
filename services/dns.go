package services
import (
		"net"
		"strings"
		"time"
	   )


func Dns(addr string) (time.Duration, error) {
	addrs := strings.Split(addr, "//")
	if len(addrs) > 1 {
		addr = addrs[1]
	}
	t1 := time.Now()
	_, err := net.ResolveIPAddr("ip", addr)
	if err != nil {
		return 0, err
	}
	t2 := time.Now()
	return t2.Sub(t1), nil
}
