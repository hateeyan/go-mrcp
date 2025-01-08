package main

import (
	"fmt"
	"github.com/hateeyan/go-mrcp"
	"sync"
)

type proxySession struct {
	dc            *mrcp.DialogClient
	ds            *mrcp.DialogServer
	serverChannel *mrcp.Channel
	clientChannel *mrcp.Channel
	rtps          chan []byte
}

func (p *proxySession) writeServerRTPPacket(m *mrcp.Media, rtp []byte) bool {
	buf := make([]byte, len(rtp))
	copy(buf, rtp)
	p.rtps <- buf
	return true
}

func (p *proxySession) onDialogServerMediaOpen(media *mrcp.Media) mrcp.MediaHandler {
	return mrcp.MediaHandlerFunc{
		WriteRTPPacketFunc: p.writeServerRTPPacket,
	}
}

func (p *proxySession) onDialogServerMessage(c *mrcp.Channel, msg mrcp.Message) {
	if err := p.dc.GetChannel().SendMrcpMessage(msg); err != nil {
		fmt.Println("failed to send message to client:", err)
		return
	}
}

func (p *proxySession) onDialogServerChannelOpen(channel *mrcp.Channel) mrcp.ChannelHandler {
	p.serverChannel = channel
	return mrcp.ChannelHandlerFunc{
		OnMessageFunc: p.onDialogServerMessage,
	}
}

func (p *proxySession) onClientMessage(c *mrcp.Channel, msg mrcp.Message) {
	if err := p.serverChannel.SendMrcpMessage(msg); err != nil {
		fmt.Println("failed to send message to server:", err)
		return
	}
}

func (p *proxySession) onDialogServerClose() {
	if p.dc != nil {
		_ = p.dc.Close()
	}
}

func (p *proxySession) readDialogClientRTPPacket(m *mrcp.Media) ([]byte, bool) {
	select {
	case buf := <-p.rtps:
		return buf, true
	default:
		return nil, true
	}
}

func (p *proxySession) onDialogClientMediaOpen(media *mrcp.Media) mrcp.MediaHandler {
	return mrcp.MediaHandlerFunc{
		ReadRTPPacketFunc: p.readDialogClientRTPPacket,
	}
}

func (p *proxySession) onDialogClientChannelOpen(channel *mrcp.Channel) mrcp.ChannelHandler {
	return mrcp.ChannelHandlerFunc{
		OnMessageFunc: p.onClientMessage,
	}
}

type proxy struct {
	client   *mrcp.Client
	sessions sync.Map
}

func (p *proxy) OnDialogCreate(ds *mrcp.DialogServer) (mrcp.DialogHandler, error) {
	ps := &proxySession{ds: ds, rtps: make(chan []byte, 10)}

	var err error
	ps.dc, err = p.client.Dial(
		"10.9.232.246:8060",
		ds.GetChannel().GetChannelId().Resource,
		mrcp.DialogHandlerFunc{
			OnMediaOpenFunc:   ps.onDialogClientMediaOpen,
			OnChannelOpenFunc: ps.onDialogClientChannelOpen,
		},
		mrcp.WithAudioCodecs(ds.GetRemoteDesc().AudioDesc.Codecs),
	)
	if err != nil {
		return nil, err
	}

	ds.GetChannel().SetChannelId(ps.dc.GetChannel().GetChannelId())
	ds.GetLocalDesc().AudioDesc.Codecs = ps.dc.GetRemoteDesc().AudioDesc.Codecs

	return mrcp.DialogHandlerFunc{
		OnMediaOpenFunc:   ps.onDialogServerMediaOpen,
		OnChannelOpenFunc: ps.onDialogServerChannelOpen,
		OnCloseFunc:       ps.onDialogServerClose,
	}, nil
}

func main() {
	client := mrcp.Client{
		LocalAddr:  "10.9.232.246:5080",
		RtpPortMin: 22000,
		RtpPortMax: 23000,
	}
	p := &proxy{client: &client}
	if err := client.Run(); err != nil {
		fmt.Println("failed to start client:", err)
		return
	}
	defer client.Close()

	server := mrcp.Server{
		Host:       "10.9.232.246",
		SIPPort:    5060,
		MRCPPort:   1560,
		RtpPortMin: 20000,
		RtpPortMax: 21000,
		Handler:    p,
	}

	if err := server.Run(); err != nil {
		fmt.Println("failed to start server:", err)
		return
	}
	defer server.Close()
}
