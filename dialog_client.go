package mrcp

import (
	"context"
	"fmt"
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/hateeyan/go-mrcp/pkg"
	"log/slog"
)

type DialogClient struct {
	callId       string
	ldesc, rdesc Desc
	sc           *Client
	channel      *Channel
	media        *Media
	session      *sipgo.DialogClientSession
	ctx          context.Context
	cancel       context.CancelFunc
	closed       bool
	logger       *slog.Logger
}

func (c *Client) newDialog(resource Resource) (*DialogClient, error) {
	audioDesc := MediaDesc{
		Host:   c.localHost,
		Ptime:  20,
		Codecs: c.AudioCodecs,
	}
	controlDesc := ControlDesc{
		Host:           c.localHost,
		Port:           9,
		Proto:          ProtoTCP,
		SetupType:      SetupActive,
		ConnectionType: ConnectionNew,
		Resource:       resource,
	}

	switch resource {
	case ResourceSpeechrecog:
		audioDesc.Direction = DirectionSendonly
	case ResourceSpeechsynth:
		audioDesc.Direction = DirectionRecvonly
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", resource)
	}

	port, err := c.porter.get()
	if err != nil {
		return nil, err
	}
	audioDesc.Port = int(port)

	d := &DialogClient{
		callId: pkg.RandString(10),
		ldesc: Desc{
			UserAgent:   c.UserAgent,
			Host:        c.localHost,
			AudioDesc:   audioDesc,
			ControlDesc: controlDesc,
		},
		sc:     c,
		logger: c.Logger,
	}
	d.ctx, d.cancel = context.WithCancel(context.Background())
	c.dialogs.Store(d.callId, d)
	return d, nil
}

func (d *DialogClient) invite(raddr string) error {
	rhost, rport, err := sip.ParseAddr(raddr)
	if err != nil {
		return err
	}

	localSDP, err := d.ldesc.generateSDP()
	if err != nil {
		return err
	}

	recipient := sip.Uri{Host: rhost, Port: rport}
	d.session, err = d.sc.ua.Invite(
		d.ctx,
		recipient,
		localSDP,
		&sip.FromHeader{
			Address: d.sc.ua.ContactHDR.Address,
			Params:  sip.HeaderParams{"tag": pkg.RandString(5)},
		},
		&sip.ToHeader{Address: recipient},
		sip.NewHeader("Call-ID", d.callId),
		sip.NewHeader("Content-Type", "application/sdp"),
	)
	if err != nil {
		return fmt.Errorf("failed to send sip invite: %v", err)
	}
	d.session.OnState(d.handleState)

	if err := d.session.WaitAnswer(d.ctx, sipgo.AnswerOptions{OnResponse: d.onResponse}); err != nil {
		return fmt.Errorf("failed to wait for answer: %v", err)
	}

	if err := d.session.Ack(d.ctx); err != nil {
		return fmt.Errorf("failed to send ack: %v", err)
	}

	return nil
}

func (d *DialogClient) handleState(s sip.DialogState) {
	switch s {
	case sip.DialogStateEnded:
		// on receive BYE
		if err := d.Close(); err != nil {
			d.logger.Error("failed to close dialog client", "error", err)
			return
		}
	}
}

func (d *DialogClient) onBye(req *sip.Request, tx sip.ServerTransaction) error {
	return d.session.ReadBye(req, tx)
}

func (d *DialogClient) onResponse(res *sip.Response) error {
	switch res.StatusCode {
	case sip.StatusOK:
		var err error
		d.rdesc, err = parseSDP(res.Body())
		if err != nil {
			d.logger.Error("failed to parse SDP", "error", err)
			d.cancel()
			return nil
		}
	}
	return nil
}

func (d *DialogClient) GetChannel() *Channel { return d.channel }

func (d *DialogClient) Close() error {
	if d.closed {
		return nil
	}
	d.closed = true

	d.cancel()
	_ = d.media.Close()
	_ = d.channel.Close()
	if d.session != nil {
		if d.session.LoadState() == sip.DialogStateConfirmed {
			if err := d.session.Bye(context.Background()); err != nil {
				d.logger.Error("failed to send bye request", "error", err)
			}
		}
		_ = d.session.Close()
	}
	d.sc.porter.free(uint16(d.ldesc.AudioDesc.Port))
	d.sc.dialogs.Delete(d.callId)

	return nil
}
