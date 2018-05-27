// Package dhcp4d implements a DHCPv4 server.
package dhcp4d

import (
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/krolaw/dhcp4"
)

type Lease struct {
	Addr         net.IP
	HardwareAddr string
	Hostname     string
	Expiry       time.Time
}

type Handler struct {
	serverIP    net.IP
	start       net.IP // first IP address to hand out
	leaseRange  int    // number of IP addresses to hand out
	leasePeriod time.Duration
	options     dhcp4.Options
	leasesHW    map[string]*Lease
	leasesIP    map[int]*Lease

	// Leases is called whenever a new lease is handed out
	Leases func([]*Lease)
}

// TODO: restore leases from permanent storage
func NewHandler() *Handler {
	serverIP := net.IP{192, 168, 42, 1} // TODO: customizeable
	return &Handler{
		leasesHW:    make(map[string]*Lease),
		leasesIP:    make(map[int]*Lease),
		serverIP:    serverIP,
		start:       net.IP{192, 168, 42, 2},
		leaseRange:  50,
		leasePeriod: 2 * time.Hour,
		options: dhcp4.Options{
			dhcp4.OptionSubnetMask:       []byte{255, 255, 255, 0},
			dhcp4.OptionRouter:           []byte(serverIP),
			dhcp4.OptionDomainNameServer: []byte(serverIP),
			dhcp4.OptionDomainName:       []byte("lan"),
			dhcp4.OptionDomainSearch:     []byte{0x03, 'l', 'a', 'n', 0x00},
		},
	}
}

func (h *Handler) findLease() int {
	if len(h.leasesIP) < h.leaseRange {
		// Hand out a free lease
		i := rand.Intn(h.leaseRange)
		if _, ok := h.leasesIP[i]; !ok {
			return i
		}
		for i := 0; i < h.leaseRange; i++ {
			if _, ok := h.leasesIP[i]; !ok {
				return i
			}
		}
	}
	// Re-use the oldest lease
	return -1
}

// TODO: is ServeDHCP always run from the same goroutine, or do we need locking?
func (h *Handler) ServeDHCP(p dhcp4.Packet, msgType dhcp4.MessageType, options dhcp4.Options) dhcp4.Packet {
	log.Printf("got DHCP packet: %+v, msgType: %v, options: %v", p, msgType, options)
	switch msgType {
	case dhcp4.Discover:
		// Find previous lease for this HardwareAddr, if any
		// hwAddr := p.CHAddr().String()
		// if lease, ok := h.leases[hwAddr]; ok {

		// }
		free := h.findLease()
		if free == -1 {
			log.Printf("Cannot reply with DHCPOFFER: no more leases available")
			return nil // no free leases
		}
		log.Printf("start = %v, free = %v", h.start, free)
		return dhcp4.ReplyPacket(p,
			dhcp4.Offer,
			h.serverIP,
			dhcp4.IPAdd(h.start, free),
			h.leasePeriod,
			h.options.SelectOrderOrAll(options[dhcp4.OptionParameterRequestList]))

	case dhcp4.Request:
		if server, ok := options[dhcp4.OptionServerIdentifier]; ok && !net.IP(server).Equal(h.serverIP) {
			return nil // message not for this dhcp server
		}
		nak := dhcp4.ReplyPacket(p, dhcp4.NAK, h.serverIP, nil, 0, nil)
		reqIP := net.IP(options[dhcp4.OptionRequestedIPAddress])
		if reqIP == nil {
			reqIP = net.IP(p.CIAddr())
		}

		if len(reqIP) != 4 || reqIP.Equal(net.IPv4zero) {
			return nak
		}
		leaseNum := dhcp4.IPRange(h.start, reqIP) - 1
		if leaseNum < 0 || leaseNum >= h.leaseRange {
			return nak
		}

		if l, exists := h.leasesIP[leaseNum]; exists && l.HardwareAddr != p.CHAddr().String() {
			return nak // lease already in use
		}

		var hostname string
		if b, ok := options[dhcp4.OptionHostName]; ok {
			hostname = string(b)
		}

		lease := &Lease{
			Addr:         reqIP,
			HardwareAddr: p.CHAddr().String(),
			Expiry:       time.Now().Add(h.leasePeriod),
			Hostname:     hostname,
		}
		h.leasesIP[leaseNum] = lease
		h.leasesHW[lease.HardwareAddr] = lease
		if h.Leases != nil {
			var leases []*Lease
			for _, l := range h.leasesIP {
				leases = append(leases, l)
			}
			h.Leases(leases)
		}
		return dhcp4.ReplyPacket(p, dhcp4.ACK, h.serverIP, reqIP, h.leasePeriod,
			h.options.SelectOrderOrAll(options[dhcp4.OptionParameterRequestList]))

	}
	//   1970/01/01 01:00:04 got DHCP packet: [1 1 6 0 142 216 238 39 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 164 76 200 233 19 71 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 99 130 83 99 53 1 3 54 4 192 168 42 1 50 4 192 168 42 33 12 3 120 112 115 55 18 1 28 2 3 15 6 119 12 44 47 26 121 42 121 249 33 252 42 255 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0], msgType: Request, options: map[OptionDHCPMessageType:[3] OptionServerIdentifier:[192 168 42 1] OptionHostName:[120 112 115] OptionParameterRequestList:[1 28 2 3 15 6 119 12 44 47 26 121 42 121 249 33 252 42] OptionRequestedIPAddress:[192 168 42 33]]

	return nil
}