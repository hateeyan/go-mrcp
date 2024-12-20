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
	c            *SIPClient
	session      *sipgo.DialogClientSession
	ctx          context.Context
	cancel       context.CancelFunc
	done         chan struct{}
	logger       *slog.Logger
}

func (c *SIPClient) newDialogClient() *DialogClient {
	d := &DialogClient{
		callId: pkg.RandString(10),
		ldesc: Desc{
			UserAgent: c.UserAgent,
			Host:      c.localHost,
			AudioDesc: MediaDesc{
				Host:      c.localHost,
				Port:      10000,
				Direction: DirectionSendonly,
				Ptime:     20,
				Codecs: []CodecDesc{
					{PayloadType: 0, Name: "PCMU", SampleRate: 8000},
					{PayloadType: 8, Name: "PCMA", SampleRate: 8000},
					{PayloadType: 101, Name: "telephone-event", SampleRate: 8000, FormatParams: map[string]string{"0-15": ""}},
				},
			},
			ControlDesc: ControlDesc{
				Host:           c.localHost,
				Port:           9,
				Proto:          "TCP/MRCPv2",
				SetupType:      SetupActive,
				ConnectionType: ConnectionNew,
				Resource:       ResourceSpeechrecog,
			},
		},
		c:      c,
		done:   make(chan struct{}),
		logger: c.Logger,
	}
	d.ctx, d.cancel = context.WithCancel(context.Background())
	c.dialogs.Store(d.callId, d)
	return d
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
	callIdHdr := sip.CallIDHeader(d.callId)
	d.session, err = d.c.ua.Invite(
		d.ctx,
		recipient,
		localSDP,
		&sip.FromHeader{
			Address: d.c.ua.ContactHDR.Address,
			Params:  sip.HeaderParams{"tag": pkg.RandString(5)},
		},
		&sip.ToHeader{Address: recipient},
		&callIdHdr,
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
		close(d.done)
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

func (d *DialogClient) Close() error {
	d.cancel()
	if d.session.LoadState() == sip.DialogStateConfirmed {
		if err := d.session.Bye(context.Background()); err != nil {
			d.logger.Error("failed to send bye request", "error", err)
		}
	}

	return d.session.Close()
}
