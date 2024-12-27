package mrcp

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"time"
)

type MediaHandler interface {
	// StartTx is called when starting to send RTP stream
	StartTx(m *Media, codec CodecDesc) error
	// ReadRTPPacket read a RTP packet from high-level
	// stop sending by returning false
	ReadRTPPacket(m *Media) ([]byte, bool)

	// StartRx is called when starting to receive RTP stream
	StartRx(m *Media, codec CodecDesc) error
	// WriteRTPPacket write a RTP packet to high-level
	// stop receiving by returning false
	WriteRTPPacket(m *Media, rtp []byte) bool
}

type MediaHandlerFunc struct {
	StartTxFunc        func(m *Media, codec CodecDesc) error
	ReadRTPPacketFunc  func(m *Media) ([]byte, bool)
	StartRxFunc        func(m *Media, codec CodecDesc) error
	WriteRTPPacketFunc func(m *Media, rtp []byte) bool
}

func (h MediaHandlerFunc) StartTx(m *Media, codec CodecDesc) error {
	if h.StartTxFunc != nil {
		return h.StartTxFunc(m, codec)
	}
	return nil
}

func (h MediaHandlerFunc) ReadRTPPacket(m *Media) ([]byte, bool) {
	if h.ReadRTPPacketFunc != nil {
		return h.ReadRTPPacketFunc(m)
	}
	return nil, false
}

func (h MediaHandlerFunc) StartRx(m *Media, codec CodecDesc) error {
	if h.StartRxFunc != nil {
		return h.StartRxFunc(m, codec)
	}
	return nil
}

func (h MediaHandlerFunc) WriteRTPPacket(m *Media, rtp []byte) bool {
	if h.WriteRTPPacketFunc != nil {
		return h.WriteRTPPacketFunc(m, rtp)
	}
	return false
}

type Media struct {
	conn                   *net.UDPConn
	remote                 *net.UDPAddr
	remoteVerified         bool
	laudioDesc, raudioDesc MediaDesc
	// preferred audio codec
	audioCodec CodecDesc
	// preferred telephone-event codec
	eventCodec CodecDesc
	handler    MediaHandler
	logger     *slog.Logger
}

func (d *DialogClient) initMedia(handler MediaHandler) error {
	d.media = &Media{
		remote: &net.UDPAddr{
			IP:   net.ParseIP(d.rdesc.AudioDesc.Host),
			Port: d.rdesc.AudioDesc.Port,
		},
		laudioDesc: d.ldesc.AudioDesc,
		raudioDesc: d.rdesc.AudioDesc,
		logger:     d.logger,
	}
	if err := d.media.negotiateCodecs(d.ldesc.AudioDesc.Codecs, d.rdesc.AudioDesc.Codecs); err != nil {
		return err
	}

	return d.media.start(handler)
}

func (d *DialogServer) newMedia() error {
	d.media = &Media{
		remote: &net.UDPAddr{
			IP:   net.ParseIP(d.rdesc.AudioDesc.Host),
			Port: d.rdesc.AudioDesc.Port,
		},
		laudioDesc: d.ldesc.AudioDesc,
		raudioDesc: d.rdesc.AudioDesc,
		logger:     d.logger,
	}
	if err := d.media.negotiateCodecs(d.ldesc.AudioDesc.Codecs, d.rdesc.AudioDesc.Codecs); err != nil {
		return err
	}
	d.ldesc.AudioDesc.Codecs = []CodecDesc{d.media.audioCodec}
	if d.media.eventCodec.Name != "" {
		d.ldesc.AudioDesc.Codecs = append(d.ldesc.AudioDesc.Codecs, d.media.eventCodec)
	}

	if d.handler != nil {
		d.media.handler = d.handler.OnMediaOpen(d.media)
		if err := d.media.start(d.media.handler); err != nil {
			return err
		}
	}

	return nil
}

func (m *Media) start(handler MediaHandler) error {
	var err error
	m.conn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(m.laudioDesc.Host), Port: m.laudioDesc.Port})
	if err != nil {
		return err
	}
	m.handler = handler

	if m.laudioDesc.Direction == DirectionRecvonly || m.laudioDesc.Direction == DirectionSendrecv {
		if err := m.handler.StartRx(m, m.audioCodec); err != nil {
			return err
		}
		go m.startReadMedia()
	}
	if m.laudioDesc.Direction == DirectionSendonly || m.laudioDesc.Direction == DirectionSendrecv {
		if err := m.handler.StartTx(m, m.audioCodec); err != nil {
			return err
		}
		go m.startSendMedia(m.laudioDesc.Ptime)
	}
	return nil
}

func (m *Media) negotiateCodecs(lcodecs, rcodecs []CodecDesc) error {
	// audio codec
loop:
	for _, rcodec := range rcodecs {
		for _, lcodec := range lcodecs {
			if rcodec.equal(lcodec) {
				m.audioCodec = rcodec
				break loop
			}
		}
	}
	if m.audioCodec.Name == "" {
		return errors.New("no available audio codec")
	}

	// telephone-event codec
	for _, rcodec := range rcodecs {
		if rcodec.Name == CodecTelephoneEvent && rcodec.SampleRate == m.audioCodec.SampleRate {
			m.eventCodec = rcodec
			break
		}
	}
	if m.eventCodec.Name == "" {
		for _, lcodec := range lcodecs {
			if lcodec.Name == CodecTelephoneEvent && lcodec.SampleRate == m.audioCodec.SampleRate {
				m.eventCodec = lcodec
				break
			}
		}
	}
	return nil
}

func (m *Media) startReadMedia() {
	buf := make([]byte, 1500)
	for {
		n, addr, err := m.conn.ReadFromUDP(buf)
		if err != nil {
			if err != io.EOF {
				m.logger.Error("failed to read media", "error", err)
			}
			break
		}
		if !m.remoteVerified {
			m.remote = addr
			m.remoteVerified = true
		}

		if ok := m.handler.WriteRTPPacket(m, buf[:n]); !ok {
			break
		}
	}
}

func (m *Media) startSendMedia(ptime int) {
	t := time.NewTicker(time.Duration(ptime) * time.Millisecond)
	for range t.C {
		data, ok := m.handler.ReadRTPPacket(m)
		if !ok {
			break
		}

		if _, err := m.conn.WriteToUDP(data, m.remote); err != nil {
			m.logger.Error("failed to send media", "error", err)
			break
		}
	}
}

func (m *Media) LocalAudioDesc() MediaDesc  { return m.laudioDesc }
func (m *Media) RemoteAudioDesc() MediaDesc { return m.raudioDesc }

func (m *Media) Close() error {
	if m == nil {
		return nil
	}
	if m.conn != nil {
		_ = m.conn.Close()
	}
	return nil
}
