package http

import "net"

func preferredHostIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	var candidates []net.IP
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch a := addr.(type) {
			case *net.IPNet:
				ip = a.IP
			case *net.IPAddr:
				ip = a.IP
			default:
				continue
			}

			ip = ip.To4()
			if ip == nil {
				continue
			}
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}

			candidates = append(candidates, ip)
		}
	}

	for _, ip := range candidates {
		if ip[0] == 192 && ip[1] == 168 && ip[2] == 1 {
			return ip.String(), nil
		}
	}
	for _, ip := range candidates {
		if ip[0] == 192 && ip[1] == 168 {
			return ip.String(), nil
		}
	}
	for _, ip := range candidates {
		if isPrivateIPv4(ip) {
			return ip.String(), nil
		}
	}
	if len(candidates) > 0 {
		return candidates[0].String(), nil
	}
	return "", nil
}

func isPrivateIPv4(ip net.IP) bool {
	if len(ip) != 4 {
		ip = ip.To4()
	}
	if len(ip) != 4 {
		return false
	}

	if ip[0] == 10 {
		return true
	}
	if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
		return true
	}
	if ip[0] == 192 && ip[1] == 168 {
		return true
	}
	return false
}
