package discovery

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// DiscoveredHost represents a potential NSM instance found on the network
type DiscoveredHost struct {
	IP   string
	Port int
}

// Scanner scans the local subnet for NSM instances
type Scanner struct {
	port       int
	overrideIP string
}

// NewScanner creates a new scanner for the specified port
func NewScanner(port int, overrideIP string) *Scanner {
	return &Scanner{
		port:       port,
		overrideIP: overrideIP,
	}
}

// Scan identifies the local subnet and scans for open ports
func (s *Scanner) Scan(ctx context.Context) (<-chan DiscoveredHost, error) {
	results := make(chan DiscoveredHost)

	if s.overrideIP != "" {
		go func() {
			defer close(results)
			ip := net.ParseIP(s.overrideIP)
			if ip == nil {
				log.Printf("Invalid override IP: %s", s.overrideIP)
				return
			}
			// Create /24 subnet around the override IP
			// We assume /24 is the most common case for this manual override
			ipv4 := ip.To4()
			if ipv4 == nil {
				log.Printf("Override IP must be IPv4: %s", s.overrideIP)
				return
			}
			
			mask := net.CIDRMask(24, 32)
			// Apply mask to get network address
			networkIP := make(net.IP, 4)
			for i := 0; i < 4; i++ {
				networkIP[i] = ipv4[i] & mask[i]
			}
			
			ipNet := &net.IPNet{IP: networkIP, Mask: mask}
			log.Printf("Scanning override subnet %s", ipNet.String())
			s.scanSubnet(ctx, ipNet, results)
		}()
		return results, nil
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w", err)
	}

	var wg sync.WaitGroup

	// Find suitable interfaces and scan their subnets
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			log.Printf("Error getting addresses for interface %s: %v", i.Name, err)
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			var ipNet *net.IPNet

			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
				ipNet = v
			case *net.IPAddr:
				ip = v.IP
				// Create a default mask if not provided (though usually IPNet is returned)
				mask := ip.DefaultMask()
				ipNet = &net.IPNet{IP: ip, Mask: mask}
			}

			if ip == nil || ip.To4() == nil {
				continue
			}

			// Skip link-local
			if ip.IsLinkLocalUnicast() {
				continue
			}

			log.Printf("Scanning subnet %s on interface %s", ipNet.String(), i.Name)

			// Scan this subnet
			wg.Add(1)
			go func(network *net.IPNet) {
				defer wg.Done()
				s.scanSubnet(ctx, network, results)
			}(ipNet)
		}
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results, nil
}

func (s *Scanner) scanSubnet(ctx context.Context, ipNet *net.IPNet, results chan<- DiscoveredHost) {
	// Simple iteration over the subnet
	
	// Convert IP to 4-byte representation
	ip := ipNet.IP.To4()
	if ip == nil {
		return
	}

	mask := ipNet.Mask
	if len(mask) != 4 {
		return
	}

	// Calculate start and end IP
	start := make(net.IP, 4)
	end := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		start[i] = ip[i] & mask[i]
		end[i] = ip[i] | ^mask[i]
	}

	// Iterate (skip network and broadcast)
	// We'll use a semaphore to limit concurrency
	sem := make(chan struct{}, 50) // 50 concurrent scans
	var wg sync.WaitGroup

	// Simplified iteration:
	// Convert to uint32, iterate, convert back.
	startVal := binaryIP(start)
	endVal := binaryIP(end)
	
	count := endVal - startVal
	if count > 512 {
		// Limit scan to 512 hosts to avoid flooding large subnets
		// or just scan the /24 surrounding the local IP
		// For now, let's stick to /24 logic if mask is /24 or larger
		ones, _ := ipNet.Mask.Size()
		if ones < 23 {
			// Too big, just scan the local /24
			// Reset start/end to local /24
			startVal = (binaryIP(ip) & 0xFFFFFF00)
			endVal = startVal | 0xFF
		}
	}

	for i := startVal + 1; i < endVal; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		currentIP := make(net.IP, 4)
		currentIP[0] = byte(i >> 24)
		currentIP[1] = byte(i >> 16)
		currentIP[2] = byte(i >> 8)
		currentIP[3] = byte(i)

		// Skip own IP? Maybe not, useful to discover self if needed, but usually we skip.
		if currentIP.Equal(ip) {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(targetIP string) {
			defer wg.Done()
			defer func() { <-sem }()
			
			if s.checkPort(ctx, targetIP) {
				log.Printf("Found active host: %s:%d", targetIP, s.port)
				select {
				case results <- DiscoveredHost{IP: targetIP, Port: s.port}:
				case <-ctx.Done():
				}
			}
		}(currentIP.String())
	}
	
	wg.Wait()
}

func (s *Scanner) checkPort(ctx context.Context, ip string) bool {
	d := net.Dialer{Timeout: 500 * time.Millisecond}
	conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", ip, s.port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func binaryIP(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}
