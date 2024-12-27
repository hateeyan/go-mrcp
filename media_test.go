package mrcp

import (
	"reflect"
	"testing"
)

func TestMedia_negotiateCodecs(t *testing.T) {
	type args struct {
		lcodecs []CodecDesc
		rcodecs []CodecDesc
	}
	tests := []struct {
		name           string
		args           args
		wantAudioCodec CodecDesc
		wantEventCodec CodecDesc
		wantErr        bool
	}{
		{
			name: "one codec",
			args: args{
				lcodecs: []CodecDesc{
					{PayloadType: 0, Name: "PCMU", SampleRate: 8000},
					{PayloadType: 8, Name: "PCMA", SampleRate: 8000},
					{PayloadType: 103, Name: CodecTelephoneEvent, SampleRate: 8000, FormatParams: map[string]string{"0-15": ""}},
				},
				rcodecs: []CodecDesc{
					{PayloadType: 8, Name: "PCMA", SampleRate: 8000},
					{PayloadType: 101, Name: CodecTelephoneEvent, SampleRate: 8000, FormatParams: map[string]string{"0-15": ""}},
				},
			},
			wantAudioCodec: CodecDesc{PayloadType: 8, Name: "PCMA", SampleRate: 8000},
			wantEventCodec: CodecDesc{PayloadType: 101, Name: CodecTelephoneEvent, SampleRate: 8000, FormatParams: map[string]string{"0-15": ""}},
			wantErr:        false,
		},
		{
			name: "multiple event codecs",
			args: args{
				lcodecs: []CodecDesc{
					{PayloadType: 0, Name: "PCMU", SampleRate: 8000},
					{PayloadType: 8, Name: "PCMA", SampleRate: 8000},
					{PayloadType: 101, Name: CodecTelephoneEvent, SampleRate: 16000, FormatParams: map[string]string{"0-15": ""}},
					{PayloadType: 102, Name: CodecTelephoneEvent, SampleRate: 8000, FormatParams: map[string]string{"0-15": ""}},
				},
				rcodecs: []CodecDesc{
					{PayloadType: 8, Name: "PCMA", SampleRate: 8000},
					{PayloadType: 101, Name: CodecTelephoneEvent, SampleRate: 8000, FormatParams: map[string]string{"0-15": ""}},
				},
			},
			wantAudioCodec: CodecDesc{PayloadType: 8, Name: "PCMA", SampleRate: 8000},
			wantEventCodec: CodecDesc{PayloadType: 101, Name: CodecTelephoneEvent, SampleRate: 8000, FormatParams: map[string]string{"0-15": ""}},
			wantErr:        false,
		},
		{
			name: "without remote event codecs",
			args: args{
				lcodecs: []CodecDesc{
					{PayloadType: 0, Name: "PCMU", SampleRate: 8000},
					{PayloadType: 8, Name: "PCMA", SampleRate: 8000},
					{PayloadType: 101, Name: CodecTelephoneEvent, SampleRate: 16000, FormatParams: map[string]string{"0-15": ""}},
					{PayloadType: 102, Name: CodecTelephoneEvent, SampleRate: 8000, FormatParams: map[string]string{"0-15": ""}},
				},
				rcodecs: []CodecDesc{
					{PayloadType: 8, Name: "PCMA", SampleRate: 8000},
				},
			},
			wantAudioCodec: CodecDesc{PayloadType: 8, Name: "PCMA", SampleRate: 8000},
			wantEventCodec: CodecDesc{PayloadType: 102, Name: CodecTelephoneEvent, SampleRate: 8000, FormatParams: map[string]string{"0-15": ""}},
			wantErr:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Media{}
			if err := m.negotiateCodecs(tt.args.lcodecs, tt.args.rcodecs); (err != nil) != tt.wantErr {
				t.Errorf("negotiateCodecs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(m.audioCodec, tt.wantAudioCodec) {
				t.Errorf("audioCodec = %v, want %v", m.audioCodec, tt.wantAudioCodec)
				return
			}
			if !reflect.DeepEqual(m.eventCodec, tt.wantEventCodec) {
				t.Errorf("eventCodec = %v, want %v", m.eventCodec, tt.wantEventCodec)
				return
			}
		})
	}
}
