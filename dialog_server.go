package mrcp

import (
	"context"
	"fmt"
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"log/slog"
)

type DialogServer struct {
	callId       string
	ldesc, rdesc Desc
	ss           *Server
	channel      *Channel
	media        *Media
	session      *sipgo.DialogServerSession
	handler      DialogHandler
	ctx          context.Context
	cancel       context.CancelFunc
	closed       bool
	logger       *slog.Logger
}

func (s *Server) newDialog(session *sipgo.DialogServerSession) *DialogServer {
	d := &DialogServer{
		callId: session.InviteRequest.CallID().Value(),
		ldesc: Desc{
			UserAgent: s.UserAgent,
			Host:      s.Host,
			AudioDesc: MediaDesc{
				Host:   s.Host,
				Ptime:  20,
				Codecs: s.AudioCodecs,
			},
			ControlDesc: ControlDesc{
				Host:           s.Host,
				Port:           s.MRCPPort,
				Proto:          ProtoTCP,
				SetupType:      SetupPassive,
				ConnectionType: ConnectionNew,
			},
		},
		ss:      s,
		channel: s.getOrCreateChannel(),
		session: session,
		logger:  s.Logger,
	}
	d.ldesc.ControlDesc.Channel = d.channel.GetChannelId()
	d.ctx, d.cancel = context.WithCancel(context.Background())
	s.dialogs.Store(d.callId, d)
	s.dialogs.Store(d.channel.GetChannelId().Id, d)

	if s.Handler != nil {
		d.handler = s.Handler.OnDialogCreate(d)
	}
	return d
}

func (d *DialogServer) onInvite(req *sip.Request, tx sip.ServerTransaction) error {
	d.session.OnState(d.handleState)
	if err := d.session.Respond(sip.StatusTrying, "Trying", nil); err != nil {
		return fmt.Errorf("failed to respond 100 trying: %v", err)
	}

	rdesc, err := parseSDP(req.Body())
	if err != nil {
		if err := d.session.Respond(sip.StatusInternalServerError, "Internal Server Error", nil); err != nil {
			d.logger.Error("failed to respond 500 internal server error", "error", err)
		}
		return fmt.Errorf("failed to parse sdp: %v", err)
	}
	d.rdesc = rdesc

	d.setResource(rdesc.ControlDesc.Resource)
	switch rdesc.ControlDesc.Resource {
	case ResourceSpeechrecog:
		d.ldesc.AudioDesc.Direction = DirectionRecvonly
	case ResourceSpeechsynth:
		d.ldesc.AudioDesc.Direction = DirectionSendonly
	default:
		if err := d.session.Respond(sip.StatusInternalServerError, "Internal Server Error", nil); err != nil {
			d.logger.Error("failed to respond 500 internal server error", "error", err)
		}
		return fmt.Errorf("unsupported resource type: %s", rdesc.ControlDesc.Resource)
	}

	port, err := d.ss.porter.get()
	if err != nil {
		if err := d.session.Respond(sip.StatusInternalServerError, "Internal Server Error", nil); err != nil {
			d.logger.Error("failed to respond 500 internal server error", "error", err)
		}
		return err
	}
	d.ldesc.AudioDesc.Port = int(port)

	if err := d.newMedia(); err != nil {
		if err := d.session.Respond(sip.StatusInternalServerError, "Internal Server Error", nil); err != nil {
			d.logger.Error("failed to respond 500 internal server error", "error", err)
		}
		return err
	}
	localSDP, err := d.ldesc.generateSDP()
	if err != nil {
		if err := d.session.Respond(sip.StatusInternalServerError, "Internal Server Error", nil); err != nil {
			d.logger.Error("failed to respond 500 internal server error", "error", err)
		}
		return err
	}
	if err := d.session.RespondSDP(localSDP); err != nil {
		return fmt.Errorf("failed to respond 200 ok: %v", err)
	}

	return nil
}

// TODO: support modify descs
func (d *DialogServer) onReInvite(req *sip.Request, tx sip.ServerTransaction) error {
	if err := d.session.ReadRequest(req, tx); err != nil {
		return err
	}

	res := sip.NewResponseFromRequest(req, sip.StatusTrying, "Trying", nil)
	if err := tx.Respond(res); err != nil {
		return fmt.Errorf("failed to respond 100 trying: %v", err)
	}

	rdesc, err := parseSDP(req.Body())
	if err != nil {
		res = sip.NewResponseFromRequest(req, sip.StatusInternalServerError, "Internal Server Error", nil)
		if err := tx.Respond(res); err != nil {
			d.logger.Error("failed to respond 500 internal server error", "error", err)
		}
		return fmt.Errorf("failed to parse sdp: %v", err)
	}
	d.rdesc = rdesc

	if rdesc.ControlDesc.Port == 0 {
		d.ldesc.ControlDesc.Port = 0
		_ = d.channel.Close()
	}
	if rdesc.AudioDesc.Port == 0 {
		d.ldesc.AudioDesc.Port = 0
		d.ldesc.AudioDesc.Direction = DirectionInactive
		_ = d.media.Close()
	}

	localSDP, err := d.ldesc.generateSDP()
	if err != nil {
		res = sip.NewResponseFromRequest(req, sip.StatusInternalServerError, "Internal Server Error", nil)
		if err := tx.Respond(res); err != nil {
			d.logger.Error("failed to respond 500 internal server error", "error", err)
		}
		return err
	}
	res = sip.NewSDPResponseFromRequest(req, localSDP)
	if err := tx.Respond(res); err != nil {
		return fmt.Errorf("failed to respond 200 ok: %v", err)
	}
	return nil
}

func (d *DialogServer) onBye(req *sip.Request, tx sip.ServerTransaction) error {
	if err := d.session.ReadBye(req, tx); err != nil {
		return err
	}
	return nil
}

func (d *DialogServer) handleState(s sip.DialogState) {
	switch s {
	case sip.DialogStateEnded:
		// on receive BYE
		if err := d.Close(); err != nil {
			d.logger.Error("failed to close dialog server", "error", err)
			return
		}
	}
}

func (d *DialogServer) setResource(resource Resource) {
	d.ldesc.ControlDesc.Channel.Resource = resource
	d.channel.setResource(resource)
}

func (d *DialogServer) Close() error {
	if d.closed {
		return nil
	}
	d.closed = true

	d.cancel()
	_ = d.media.Close()
	_ = d.channel.Close()
	if d.session.LoadState() == sip.DialogStateConfirmed {
		if err := d.session.Bye(context.Background()); err != nil {
			d.logger.Error("failed to send bye request", "error", err)
		}
	}
	d.ss.porter.free(uint16(d.ldesc.AudioDesc.Port))
	d.ss.dialogs.Delete(d.callId)
	d.ss.dialogs.Delete(d.channel.GetChannelId().Id)

	return d.session.Close()
}
