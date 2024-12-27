package mrcp

import (
	"errors"
	"sync"
	"sync/atomic"
)

const (
	defaultUserAgent  = "go-mrcp"
	defaultRtpPortMin = 20000
	defaultRtpPortMax = 40000
)

var (
	ErrNoFreePorts = errors.New("no free rtp ports")
)

type porter struct {
	ports      sync.Map
	nextPort   uint16
	portsUsed  atomic.Int64
	portsRange uint16
	portMin    uint16
	portMax    uint16
}

func newPorter(portMin, portMax uint16) (*porter, error) {
	if int(portMax-portMin) < 2 {
		return nil, errors.New("invalid port range")
	}

	return &porter{
		nextPort:   portMin,
		portsRange: portMax - portMin,
		portMin:    portMin,
		portMax:    portMax,
	}, nil
}

// get get a free RTP and RTCP port pair.
func (p *porter) get() (uint16, error) {
	if p.portsRange-uint16(p.portsUsed.Load()) < 2 {
		return 0, ErrNoFreePorts
	}

	for {
		port := p.nextPort
		_, inuse := p.ports.LoadOrStore(port, struct{}{})
		p.nextPort += 2
		if p.nextPort >= p.portMax {
			p.nextPort = p.portMin
		}
		if inuse {
			continue
		}
		p.portsUsed.Add(2)
		return port, nil
	}
}

func (p *porter) free(port uint16) {
	p.ports.Delete(port)
	p.portsUsed.Add(-2)
}
