package mrcp

import (
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"log/slog"
	"sync"
)

type Client struct {
	// Host local host
	// default: 127.0.0.1
	Host string
	// SIPPort SIP server port
	// default: 5060
	SIPPort int
	// UserAgent SIP User-Agent
	UserAgent string
	// AudioCodecs audio codecs
	// Default: defaultAudioCodecs
	AudioCodecs []CodecDesc
	// RtpPortMin RtpPortMax RTP port range
	// Default: [20000, 40000)
	RtpPortMin, RtpPortMax uint16
	// Logger
	// Default: slog.Default
	Logger *slog.Logger

	// internal
	porter  *porter
	ua      sipgo.DialogUA
	dialogs sync.Map
}

func (c *Client) Run() error {
	if c.Host == "" {
		c.Host = "127.0.0.1"
	}
	if c.SIPPort == 0 {
		c.SIPPort = 5060
	}
	if c.UserAgent == "" {
		c.UserAgent = defaultUserAgent
	}
	if len(c.AudioCodecs) == 0 {
		c.AudioCodecs = defaultAudioCodecs
	}
	if c.RtpPortMin == 0 {
		c.RtpPortMin = defaultRtpPortMin
	}
	if c.RtpPortMax == 0 {
		c.RtpPortMax = defaultRtpPortMax
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}

	var err error
	c.porter, err = newPorter(c.RtpPortMin, c.RtpPortMax)
	if err != nil {
		return err
	}

	ua, err := sipgo.NewUA()
	if err != nil {
		return err
	}
	ua.TransactionLayer().OnRequest(c.onRequest)

	client, err := sipgo.NewClient(ua, sipgo.WithClientHostname(c.Host), sipgo.WithClientPort(c.SIPPort))
	if err != nil {
		_ = ua.Close()
		return err
	}

	c.ua = sipgo.DialogUA{
		Client:     client,
		ContactHDR: sip.ContactHeader{Address: sip.Uri{User: c.UserAgent, Host: c.Host, Port: c.SIPPort}},
	}

	return nil
}

func (c *Client) Dial(
	raddr string,
	resource Resource,
	handler DialogHandler,
	opts ...DialogClientOptionFunc,
) (*DialogClient, error) {
	dc, err := c.newDialog(resource, handler, opts...)
	if err != nil {
		return nil, err
	}
	if err := dc.invite(raddr); err != nil {
		_ = dc.Close()
		return nil, err
	}
	if err := dc.initMedia(); err != nil {
		_ = dc.Close()
		return nil, err
	}
	if err := dc.dialMRCPServer(); err != nil {
		_ = dc.Close()
		return nil, err
	}
	return dc, nil
}

func (c *Client) onBye(req *sip.Request, tx sip.ServerTransaction) {
	got, ok := c.dialogs.Load(req.CallID().Value())
	if !ok {
		if err := tx.Respond(sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)); err != nil {
			c.Logger.Error("failed to respond to bye", "callId", req.CallID().Value(), "error", err)
		}
		return
	}

	dialog := got.(*DialogClient)
	if err := dialog.onBye(req, tx); err != nil {
		c.Logger.Error("failed to respond to bye", "callId", req.CallID().Value(), "error", err)
		return
	}
}

func (c *Client) onRequest(req *sip.Request, tx sip.ServerTransaction) {
	switch req.Method {
	case sip.BYE:
		c.onBye(req, tx)
	default:
		c.Logger.Warn("SIP request handler not found", "method", req.Method)
		res := sip.NewResponseFromRequest(req, 405, "Method Not Allowed", nil)
		if err := tx.Respond(res); err != nil {
			c.Logger.Error("respond '405 Method Not Allowed' failed", "error", err)
			return
		}
	}

	if tx != nil {
		tx.Terminate()
	}
}

func (c *Client) Close() error {
	_ = c.ua.Client.Close()
	_ = c.ua.Client.UserAgent.Close()
	return nil
}
