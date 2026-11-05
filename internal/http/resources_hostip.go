package http

import "net"

type HostIPCandidate struct {
	Interface string `json:"interface"`
	IP        string `json:"ip"`
}

// detectHostIPs lists the usable IPv4 addresses of this machine and picks the
// one most likely to be the LAN address. The full list is offered in the UI so
// the user can pick a different one when the guess is wrong.
func detectHostIPs() (string, []HostIPCandidate, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", nil, err
	}

	type candidate struct {
		iface string
		ip    net.IP
	}
	var candidates []candidate
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

			candidates = append(candidates, candidate{iface: iface.Name, ip: ip})
		}
	}

	// Container bridges (docker0, br-*, veth*, ...) are never the address you
	// want to open the dashboard on. Drop them unless nothing else is left.
	physical := candidates[:0:0]
	for _, c := range candidates {
		if !skipNetInterface(c.iface) {
			physical = append(physical, c)
		}
	}
	if len(physical) > 0 {
		candidates = physical
	}

	all := make([]HostIPCandidate, 0, len(candidates))
	for _, c := range candidates {
		all = append(all, HostIPCandidate{Interface: c.iface, IP: c.ip.String()})
	}

	preferred := ""
	pick := func(match func(net.IP) bool) bool {
		for _, c := range candidates {
			if match(c.ip) {
				preferred = c.ip.String()
				return true
			}
		}
		return false
	}
	switch {
	case pick(func(ip net.IP) bool { return ip[0] == 192 && ip[1] == 168 && ip[2] == 1 }):
	case pick(func(ip net.IP) bool { return ip[0] == 192 && ip[1] == 168 }):
	case pick(isPrivateIPv4):
	case len(candidates) > 0:
		preferred = candidates[0].ip.String()
	}
	return preferred, all, nil
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
