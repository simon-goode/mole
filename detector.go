package main

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

func detectIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() {
				continue
			}

			ipv4 := ipnet.IP.To4()
			if ipv4 == nil {
				continue
			}

			return ipv4.String(), nil
		}
	}

	return "", fmt.Errorf("no suitable network interface found")
}

func checkConnectivity(ip string, port int, token string) bool {
	url := fmt.Sprintf("http://%s:%d/%s/health", ip, port, token)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}
