package mrcp

import (
	"reflect"
	"testing"
)

func Test_parseSDP(t *testing.T) {
	type args struct {
		raw []byte
	}
	tests := []struct {
		name    string
		args    args
		want    Desc
		wantErr bool
	}{
		{
			name: "sdp",
			args: args{raw: []byte("v=0\r\no=UniMRCPServer 5710209595858788961 7814554407398160305 IN IP4 10.29.0.87\r\ns=-\r\nc=IN IP4 10.29.0.87\r\nt=0 0\r\nm=application 7230 TCP/MRCPv2 1\r\na=setup:passive\r\na=connection:new\r\na=channel:24208d6b89a1403f@speechrecog\r\na=cmid:1\r\nm=audio 22836 RTP/AVP 0 101\r\na=rtpmap:0 PCMU/8000\r\na=rtpmap:101 telephone-event/8000\r\na=fmtp:101 0-15\r\na=recvonly\r\na=ptime:20\r\na=mid:1\r\n")},
			want: Desc{
				UserAgent: "UniMRCPServer",
				Host:      "10.29.0.87",
				AudioDesc: MediaDesc{
					Host:      "10.29.0.87",
					Port:      22836,
					Direction: DirectionRecvonly,
					Ptime:     20,
					Codecs: []CodecDesc{
						{PayloadType: 0, Name: "PCMU", SampleRate: 8000},
						{PayloadType: 101, Name: "telephone-event", SampleRate: 8000, FormatParams: map[string]string{"0-15": ""}},
					},
				},
				ControlDesc: ControlDesc{
					Host:           "10.29.0.87",
					Port:           7230,
					Proto:          "TCP/MRCPv2",
					SetupType:      SetupPassive,
					ConnectionType: ConnectionNew,
					Channel:        "24208d6b89a1403f@speechrecog",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSDP(tt.args.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSDP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseSDP() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDesc_generateSDP(t *testing.T) {
	type fields struct {
		Host        string
		UserAgent   string
		AudioDesc   MediaDesc
		ControlDesc ControlDesc
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{
			name: "sdp",
			fields: fields{
				Host:      "127.0.0.1",
				UserAgent: "go-mrcp",
				AudioDesc: MediaDesc{
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
					Port:           9,
					Proto:          ProtoTCP,
					SetupType:      SetupActive,
					ConnectionType: ConnectionNew,
					Resource:       ResourceSpeechrecog,
				},
			},
			want:    "v=0\r\no=go-mrcp 0 0 IN IP4 127.0.0.1\r\ns=-\r\nc=IN IP4 127.0.0.1\r\nt=0 0\r\nm=application 9 TCP/MRCPv2 1\r\na=setup:active\r\na=connection:new\r\na=resource:speechrecog\r\na=cmid:1\r\nm=audio 10000 RTP/AVP 0 8 101\r\na=sendonly\r\na=ptime:20\r\na=mid:1\r\na=rtpmap:0 PCMU/8000\r\na=rtpmap:8 PCMA/8000\r\na=rtpmap:101 telephone-event/8000\r\na=fmtp:101 0-15\r\n",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Desc{
				Host:        tt.fields.Host,
				UserAgent:   tt.fields.UserAgent,
				AudioDesc:   tt.fields.AudioDesc,
				ControlDesc: tt.fields.ControlDesc,
			}
			got, err := d.generateSDP()
			if (err != nil) != tt.wantErr {
				t.Errorf("generateSDP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if string(got) != tt.want {
				t.Errorf("generateSDP() got = %v, want %v", string(got), tt.want)
			}
		})
	}
}
