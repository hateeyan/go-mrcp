package mrcp

import (
	"context"
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/hateeyan/go-mrcp/pkg"
	"log/slog"
	"net"
	"strconv"
	"sync"
)

type ServerHandler interface {
	OnDialogCreate(d *DialogServer) DialogHandler
}

type ServerHandlerFunc struct {
	OnDialogCreateFunc func(d *DialogServer) DialogHandler
}

func (h ServerHandlerFunc) OnDialogCreate(d *DialogServer) DialogHandler {
	if h.OnDialogCreateFunc != nil {
		return h.OnDialogCreateFunc(d)
	}
	return nil
}

type Server struct {
	// Host local host
	// default: 127.0.0.1
	Host string
	// SIPPort SIP server port
	// default: 5060
	SIPPort int
	// MRCPPort MRCP server port
	// default: 1544
	MRCPPort int
	// UserAgent SIP User-Agent
	UserAgent string
	// AudioCodecs audio codecs
	// Default: defaultAudioCodecs
	AudioCodecs []CodecDesc
	// RtpPortMin RtpPortMax RTP port range
	// Default: [20000, 40000)
	RtpPortMin, RtpPortMax uint16
	// Handler handler
	Handler ServerHandler
	// Logger
	// Default: slog.Default
	Logger *slog.Logger

	// internal
	porter  *porter
	ua      sipgo.DialogUA
	dialogs sync.Map
}

func (s *Server) Run() error {
	if s.Host == "" {
		s.Host = "127.0.0.1"
	}
	if s.SIPPort == 0 {
		s.SIPPort = 5060
	}
	if s.MRCPPort == 0 {
		s.MRCPPort = 1544
	}
	if s.UserAgent == "" {
		s.UserAgent = defaultUserAgent
	}
	if len(s.AudioCodecs) == 0 {
		s.AudioCodecs = defaultAudioCodecs
	}
	if s.RtpPortMin == 0 {
		s.RtpPortMin = defaultRtpPortMin
	}
	if s.RtpPortMax == 0 {
		s.RtpPortMax = defaultRtpPortMax
	}
	if s.Logger == nil {
		s.Logger = slog.Default()
	}

	var err error
	s.porter, err = newPorter(s.RtpPortMin, s.RtpPortMax)
	if err != nil {
		return err
	}

	ua, err := sipgo.NewUA()
	if err != nil {
		return err
	}

	sipServer, err := sipgo.NewServer(ua)
	if err != nil {
		_ = ua.Close()
		return err
	}
	sipServer.OnInvite(s.onInvite)
	sipServer.OnAck(s.onAck)
	sipServer.OnBye(s.onBye)

	client, err := sipgo.NewClient(ua, sipgo.WithClientHostname(s.Host), sipgo.WithClientPort(s.SIPPort))
	if err != nil {
		_ = sipServer.Close()
		_ = ua.Close()
		return err
	}

	s.ua = sipgo.DialogUA{
		Client:     client,
		ContactHDR: sip.ContactHeader{Address: sip.Uri{User: s.UserAgent, Host: s.Host, Port: s.SIPPort}},
	}

	if err := s.startMRCPServer(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Logger.Info("starting sip server", "listening", s.Host+":"+strconv.Itoa(s.SIPPort))
	if err := sipServer.ListenAndServe(ctx, "udp4", s.Host+":"+strconv.Itoa(s.SIPPort)); err != nil {
		_ = ua.Close()
		_ = client.Close()
		return err
	}

	return nil
}

func (s *Server) startMRCPServer() error {
	s.Logger.Info("starting mrcp server", "listening", s.Host+":"+strconv.Itoa(s.MRCPPort))
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.ParseIP(s.Host), Port: s.MRCPPort})
	if err != nil {
		return err
	}
	go func() {
		for {
			conn, err := listener.AcceptTCP()
			if err != nil {
				s.Logger.Error("failed to accept connection", "error", err)
				break
			}

			s.accept(conn, connectionHandlerFunc{OnMessageFunc: s.onMessage})
		}
	}()
	return nil
}

func (s *Server) onInvite(req *sip.Request, tx sip.ServerTransaction) {
	callId := req.CallID().Value()
	session, err := s.ua.ReadInvite(req, tx)
	if err != nil {
		s.Logger.Error("failed to read INVITE request", "call", callId, "error", err)
		return
	}
	dialog := s.newDialog(session)

	if err := dialog.onInvite(req, tx); err != nil {
		s.Logger.Error("failed to handle new dialog", "call", callId, "error", err)
		return
	}
}

func (s *Server) onAck(req *sip.Request, tx sip.ServerTransaction) {
	got, ok := s.dialogs.Load(req.CallID().Value())
	if !ok {
		return
	}
	dialog := got.(*DialogServer)
	if err := dialog.session.ReadAck(req, tx); err != nil {
		s.Logger.Error("failed to read ACK request", "call", req.CallID(), "error", err)
		return
	}
}

func (s *Server) onBye(req *sip.Request, tx sip.ServerTransaction) {
	got, ok := s.dialogs.Load(req.CallID().Value())
	if !ok {
		if err := tx.Respond(sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)); err != nil {
			s.Logger.Warn("failed to respond BYE request", "call", req.CallID(), "error", err)
		}
		return
	}
	dialog := got.(*DialogServer)
	if err := dialog.onBye(req, tx); err != nil {
		s.Logger.Error("failed to handle BYE request", "call", req.CallID(), "error", err)
		return
	}
}

func (s *Server) onMessage(c *connection, msg Message) {
	cid := parseChannelId(msg.GetHeader(HeaderChannelIdentifier))
	got, ok := s.dialogs.Load(cid.Id)
	if !ok {
		s.Logger.Warn("no such dialog", "channelId", cid.String())
		return
	}

	d := got.(*DialogServer)
	if !d.channel.bound() {
		handler := d.handler.OnChannelOpen(d.channel)
		d.channel.bind(c, handler)
	}
	d.channel.onMessage(msg)
}

func (s *Server) getOrCreateChannel() *Channel {
	// TODO: reuse channel
	id := pkg.RandString(10)
	c := &Channel{
		id:     ChannelId{Id: id},
		logger: s.Logger,
	}
	return c
}
