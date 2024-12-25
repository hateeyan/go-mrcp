package mrcp

import (
	"fmt"
	"github.com/pion/sdp/v3"
	"strconv"
	"strings"
)

const (
	ProtoTCP = "TCP/MRCPv2"
	ProtoTLS = "TCP/TLS/MRCPv2"

	SetupActive  = "active"
	SetupPassive = "passive"

	ConnectionNew      = "new"
	ConnectionExisting = "existing"
)

type Resource string

const (
	ResourceSpeechsynth Resource = "speechsynth"
	ResourceSpeechrecog Resource = "speechrecog"
)

type Direction string

const (
	DirectionSendonly Direction = "sendonly"
	DirectionRecvonly Direction = "recvonly"
	DirectionSendrecv Direction = "sendrecv"
)

type ControlDesc struct {
	// Host The connection-address in the SDP Connection field
	Host           string
	Port           int
	Proto          string
	SetupType      string
	ConnectionType string
	Channel        string
	Resource       Resource
}

var codecsMap = map[int]CodecDesc{
	0:   {PayloadType: 0, Name: "PCMU", SampleRate: 8000},
	8:   {PayloadType: 8, Name: "PCMA", SampleRate: 8000},
	101: {PayloadType: 101, Name: "telephone-event", SampleRate: 8000, FormatParams: map[string]string{"0-15": ""}},
}

type CodecDesc struct {
	PayloadType  int
	Name         string
	SampleRate   int
	FormatParams map[string]string
}

func (c CodecDesc) equal(cd CodecDesc) bool {
	return c.PayloadType == cd.PayloadType && c.Name == cd.Name && c.SampleRate == cd.SampleRate
}

func negotiateCodec(lcodecs, rcodecs []CodecDesc) (CodecDesc, bool) {
	for _, rcodec := range rcodecs {
		for _, lcodec := range lcodecs {
			if rcodec.equal(lcodec) {
				return rcodec, true
			}
		}
	}
	return CodecDesc{}, false
}

type MediaDesc struct {
	// Host The connection-address in the SDP Connection field
	Host      string
	Port      int
	Direction Direction
	Ptime     int
	Codecs    []CodecDesc
}

// Desc SDP
type Desc struct {
	// UserAgent The username in the SDP Origin field
	UserAgent string
	// Host The global connection-address in the SDP Connection field
	Host        string
	AudioDesc   MediaDesc
	ControlDesc ControlDesc
}

func parseSDP(raw []byte) (Desc, error) {
	var sd sdp.SessionDescription
	if err := sd.Unmarshal(raw); err != nil {
		return Desc{}, err
	}

	desc := Desc{
		UserAgent:   sd.Origin.Username,
		AudioDesc:   MediaDesc{},
		ControlDesc: ControlDesc{},
	}

	if sd.ConnectionInformation != nil {
		desc.Host = sd.ConnectionInformation.Address.Address
		desc.AudioDesc.Host = sd.ConnectionInformation.Address.Address
		desc.ControlDesc.Host = sd.ConnectionInformation.Address.Address
	}

	for _, md := range sd.MediaDescriptions {
		if md.MediaName.Media == "application" {
			desc.ControlDesc.Host = sd.ConnectionInformation.Address.Address
			if md.ConnectionInformation != nil {
				desc.ControlDesc.Host = md.ConnectionInformation.Address.Address
			}
			desc.ControlDesc.Port = md.MediaName.Port.Value
			desc.ControlDesc.Proto = strings.Join(md.MediaName.Protos, "/")
			for _, a := range md.Attributes {
				switch a.Key {
				case "setup":
					desc.ControlDesc.SetupType = a.Value
				case "connection":
					desc.ControlDesc.ConnectionType = a.Value
				case "channel":
					desc.ControlDesc.Channel = a.Value
				case "resource":
					desc.ControlDesc.Resource = Resource(a.Value)
				}
			}
		} else if md.MediaName.Media == "audio" {
			desc.AudioDesc.Host = sd.ConnectionInformation.Address.Address
			if md.ConnectionInformation != nil {
				desc.AudioDesc.Host = md.ConnectionInformation.Address.Address
			}
			desc.AudioDesc.Port = md.MediaName.Port.Value

			for _, f := range md.MediaName.Formats {
				pt, err := strconv.Atoi(f)
				if err != nil {
					return Desc{}, fmt.Errorf("invalid format: %s", f)
				}
				codec, ok := codecsMap[pt]
				if !ok {
					continue
				}
				desc.AudioDesc.Codecs = append(desc.AudioDesc.Codecs, codec)
			}

			for _, a := range md.Attributes {
				switch a.Key {
				case "rtpmap":
					// TODO: parse rtpmap
				case "fmtp":
					// TODO: parse fmtp
				case string(DirectionSendonly), string(DirectionRecvonly), string(DirectionSendrecv):
					desc.AudioDesc.Direction = Direction(a.Key)
				case "ptime":
					got, err := strconv.Atoi(a.Value)
					if err != nil {
						return Desc{}, fmt.Errorf("invalid ptime: %s", a.Value)
					}
					desc.AudioDesc.Ptime = got
				}
			}
		}
	}

	return desc, nil
}

func (d Desc) generateSDP() ([]byte, error) {
	sd := sdp.SessionDescription{
		Version: 0,
		Origin: sdp.Origin{
			Username:       "-",
			SessionID:      0,
			SessionVersion: 0,
			NetworkType:    "IN",
			AddressType:    "IP4",
			UnicastAddress: d.Host,
		},
		SessionName: "-",
		ConnectionInformation: &sdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address:     &sdp.Address{Address: d.Host},
		},
		TimeDescriptions: []sdp.TimeDescription{{Timing: sdp.Timing{StartTime: 0, StopTime: 0}}},
		MediaDescriptions: []*sdp.MediaDescription{
			{
				MediaName: sdp.MediaName{
					Media:   "application",
					Port:    sdp.RangedPort{Value: d.ControlDesc.Port},
					Protos:  strings.Split(d.ControlDesc.Proto, "/"),
					Formats: []string{"1"},
				},
				Attributes: []sdp.Attribute{
					{Key: "setup", Value: d.ControlDesc.SetupType},
					{Key: "connection", Value: d.ControlDesc.ConnectionType},
					{Key: "resource", Value: string(d.ControlDesc.Resource)},
					{Key: "cmid", Value: "1"},
				},
			},
			{
				MediaName: sdp.MediaName{
					Media:  "audio",
					Port:   sdp.RangedPort{Value: d.AudioDesc.Port},
					Protos: []string{"RTP", "AVP"},
				},
				Attributes: []sdp.Attribute{
					{Key: string(d.AudioDesc.Direction)},
					{Key: "ptime", Value: strconv.Itoa(d.AudioDesc.Ptime)},
					{Key: "mid", Value: "1"},
				},
			},
		},
	}

	if d.UserAgent != "" {
		sd.Origin.Username = d.UserAgent
	}

	audio := sd.MediaDescriptions[1]
	for _, codec := range d.AudioDesc.Codecs {
		pt := strconv.Itoa(codec.PayloadType)
		audio.MediaName.Formats = append(audio.MediaName.Formats, pt)
		audio.Attributes = append(audio.Attributes, sdp.Attribute{Key: "rtpmap", Value: fmt.Sprintf("%d %s/%d", codec.PayloadType, codec.Name, codec.SampleRate)})

		for k, v := range codec.FormatParams {
			value := pt + " " + k
			if v != "" {
				value = value + "=" + v
			}
			audio.Attributes = append(audio.Attributes, sdp.Attribute{Key: "fmtp", Value: value})
		}
	}

	return sd.Marshal()
}
