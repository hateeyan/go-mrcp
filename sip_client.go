package mrcp

import (
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"log/slog"
	"sync"
)

type SIPClient struct {
	// LocalAddr local address
	// Format: <host>:<port>
	LocalAddr string
	// UserAgent
	UserAgent string
	// Logger
	// Default: slog.Default
	Logger *slog.Logger

	localHost string
	ua        sipgo.DialogUA
	dialogs   sync.Map
}

func (c *SIPClient) Start() error {
	if c.LocalAddr == "" {
		c.LocalAddr = "127.0.0.1:5060"
	}
	if c.UserAgent == "" {
		c.UserAgent = "go-mrcp"
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}

	lhost, lport, err := sip.ParseAddr(c.LocalAddr)
	if err != nil {
		return err
	}
	c.localHost = lhost

	ua, err := sipgo.NewUA()
	if err != nil {
		return err
	}
	ua.TransactionLayer().OnRequest(c.onRequest)

	client, err := sipgo.NewClient(ua, sipgo.WithClientHostname(lhost), sipgo.WithClientPort(lport))
	if err != nil {
		_ = ua.Close()
		return err
	}

	c.ua = sipgo.DialogUA{
		Client:     client,
		ContactHDR: sip.ContactHeader{Address: sip.Uri{User: c.UserAgent, Host: lhost, Port: lport}},
	}

	return nil
}

func (c *SIPClient) Dial(raddr string) (*DialogClient, error) {
	dc := c.newDialogClient()
	if err := dc.invite(raddr); err != nil {
		_ = dc.Close()
		return nil, err
	}
	return dc, nil
}

func (c *SIPClient) onBye(req *sip.Request, tx sip.ServerTransaction) {
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

func (c *SIPClient) onRequest(req *sip.Request, tx sip.ServerTransaction) {
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

func (c *SIPClient) Close() error {
	_ = c.ua.Client.Close()
	_ = c.ua.Client.UserAgent.Close()
	return nil
}
