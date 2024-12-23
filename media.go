package mrcp

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"time"
)

type MediaHandler struct {
	// StartTx is called when starting to send RTP stream
	StartTx func(m *Media, codec CodecDesc) error
	// ReadRTPPacket read a RTP packet from high-level
	// stop sending by returning false
	ReadRTPPacket func(m *Media) ([]byte, bool)

	// StartRx is called when starting to receive RTP stream
	StartRx func(m *Media, codec CodecDesc) error
	// WriteRTPPacket write a RTP packet to high-level
	// stop receiving by returning false
	WriteRTPPacket func(m *Media, rtp []byte) bool
}

type Media struct {
	conn                   *net.UDPConn
	remote                 *net.UDPAddr
	remoteVerified         bool
	laudioDesc, raudioDesc MediaDesc
	codec                  CodecDesc
	handler                MediaHandler
	logger                 *slog.Logger
}

func (d *DialogClient) initMedia(handler MediaHandler) error {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(d.ldesc.AudioDesc.Host), Port: d.ldesc.AudioDesc.Port})
	if err != nil {
		return err
	}

	codec, ok := negotiateCodec(d.ldesc.AudioDesc.Codecs, d.rdesc.AudioDesc.Codecs)
	if !ok {
		return errors.New("no available audio codec")
	}

	m := &Media{
		conn: conn,
		remote: &net.UDPAddr{
			IP:   net.ParseIP(d.rdesc.AudioDesc.Host),
			Port: d.rdesc.AudioDesc.Port,
		},
		laudioDesc: d.ldesc.AudioDesc,
		raudioDesc: d.rdesc.AudioDesc,
		codec:      codec,
		handler:    handler,
		logger:     d.logger,
	}
	if d.ldesc.AudioDesc.Direction == DirectionRecvonly || d.ldesc.AudioDesc.Direction == DirectionSendrecv {
		if m.handler.StartRx != nil {
			if err := m.handler.StartRx(m, codec); err != nil {
				return err
			}
		}
		go m.startReadMedia()
	}
	if d.ldesc.AudioDesc.Direction == DirectionSendonly || d.ldesc.AudioDesc.Direction == DirectionSendrecv {
		if m.handler.StartTx != nil {
			if err := m.handler.StartTx(m, codec); err != nil {
				return err
			}
		}
		go m.startSendMedia(d.ldesc.AudioDesc.Ptime)
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

		if m.handler.WriteRTPPacket != nil {
			if ok := m.handler.WriteRTPPacket(m, buf[:n]); !ok {
				break
			}
		}
	}
}

func (m *Media) startSendMedia(ptime int) {
	t := time.NewTicker(time.Duration(ptime) * time.Millisecond)
	for range t.C {
		if m.handler.ReadRTPPacket == nil {
			continue
		}
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
	return m.conn.Close()
}
