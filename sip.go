package mrcp

import "errors"

const (
	defaultUserAgent  = "go-mrcp"
	defaultRtpPortMin = 20000
	defaultRtpPortMax = 40000
)

var (
	ErrNoFreePorts = errors.New("no free rtp ports")
)

var defaultAudioCodecs = []CodecDesc{
	{PayloadType: 0, Name: "PCMU", SampleRate: 8000},
	{PayloadType: 8, Name: "PCMA", SampleRate: 8000},
	{PayloadType: 101, Name: "telephone-event", SampleRate: 8000, FormatParams: map[string]string{"0-15": ""}},
}
