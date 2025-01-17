package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/google/uuid"
	"github.com/hateeyan/go-mrcp"
	"github.com/hateeyan/go-mrcp/pkg/pcm"
	"github.com/tencentcloud/tencentcloud-speech-sdk-go/common"
	"github.com/tencentcloud/tencentcloud-speech-sdk-go/tts"
	"math/rand"
	"sync"
	"time"
)

type speechWsSynthesisListener struct {
	sessionId string
	synth     *tts.SpeechWsSynthesizer
	channel   *mrcp.Channel
	sequence  uint16
	timestamp uint32
	ssrc      uint32
	pcm       []byte
	rtp       []byte
	done      bool
	buf       bytes.Buffer
	mu        sync.Mutex
}

func newSynthesis() (*speechWsSynthesisListener, error) {
	sessionId := uuid.NewString()
	credential := common.NewCredential(secretId, secretKey)
	listener := &speechWsSynthesisListener{
		sessionId: sessionId,
		timestamp: rand.Uint32(),
		ssrc:      rand.Uint32(),
		pcm:       make([]byte, 320),
		rtp:       make([]byte, 172),
	}
	listener.synth = tts.NewSpeechWsSynthesizer(int64(appId), credential, listener)
	listener.synth.SessionId = sessionId
	listener.synth.VoiceType = 1001
	listener.synth.SampleRate = 8000
	listener.synth.Codec = "pcm"
	listener.synth.EnableSubtitle = true

	return listener, nil
}

func (l *speechWsSynthesisListener) readRTPPacket(m *mrcp.Media) ([]byte, bool) {
	if l.buf.Len() < 320 {
		if l.done {
			event := l.channel.NewEvent("SPEAK-COMPLETE", mrcp.RequestStateComplete)
			event.SetCompletionCause(l.channel.GetResource(), mrcp.SynthCompletionCauseNormal)
			if err := l.channel.SendMrcpMessage(event); err != nil {
				fmt.Println("failed to send event:", err)
			}
			return nil, false
		}
		return nil, true
	}

	l.mu.Lock()
	_, err := l.buf.Read(l.pcm)
	l.mu.Unlock()
	if err != nil {
		fmt.Println("read error:", err)
		return nil, false
	}

	// rtp header
	l.sequence++
	l.timestamp += 160
	l.rtp[0] = 0x80
	l.rtp[1] = 0
	binary.BigEndian.PutUint16(l.rtp[2:], l.sequence)
	binary.BigEndian.PutUint32(l.rtp[4:], l.timestamp)
	binary.BigEndian.PutUint32(l.rtp[8:], l.ssrc)

	// rtp payload
	// pcm to pcmu
	if err := pcm.LinearToMuLaw(l.pcm, l.rtp[12:]); err != nil {
		fmt.Println("failed to encode to pcmu:", err)
		return nil, false
	}

	return l.rtp, true
}

func (l *speechWsSynthesisListener) OnMediaOpen(media *mrcp.Media) mrcp.MediaHandler {
	return mrcp.MediaHandlerFunc{
		ReadRTPPacketFunc: l.readRTPPacket,
	}
}

func (l *speechWsSynthesisListener) onMessage(c *mrcp.Channel, msg mrcp.Message) {
	switch msg.GetName() {
	case mrcp.MethodSpeak:
		l.synth.Text = fmt.Sprintf("<speak>\n%s\n</speak>\n", string(msg.GetBody()))
		err := l.synth.Synthesis()
		if err != nil {
			fmt.Println("start synthesis error:", err)
		}
		resp := c.NewResponse(msg, 200, mrcp.RequestStateInProgress)
		if err := c.SendMrcpMessage(resp); err != nil {
			fmt.Println("failed to send response:", err)
			return
		}
	}
}

func (l *speechWsSynthesisListener) OnChannelOpen(channel *mrcp.Channel) mrcp.ChannelHandler {
	l.channel = channel
	return mrcp.ChannelHandlerFunc{
		OnMessageFunc: l.onMessage,
	}
}

func (l *speechWsSynthesisListener) OnClose() {
	l.synth.CloseConn()
	return
}

func (l *speechWsSynthesisListener) OnSynthesisStart(r *tts.SpeechWsSynthesisResponse) {
	fmt.Printf("%s|OnSynthesisStart,sessionId:%s response: %s\n", time.Now().Format("2006-01-02 15:04:05"), l.sessionId, r.ToString())
}

func (l *speechWsSynthesisListener) OnSynthesisEnd(r *tts.SpeechWsSynthesisResponse) {
	l.done = true
	fmt.Printf("%s|OnSynthesisEnd,sessionId:%s response: %s\n", time.Now().Format("2006-01-02 15:04:05"), l.sessionId, r.ToString())
}

func (l *speechWsSynthesisListener) OnAudioResult(data []byte) {
	fmt.Printf("%s|OnAudioResult,sessionId:%s\n", time.Now().Format("2006-01-02 15:04:05"), l.sessionId)
	l.mu.Lock()
	l.buf.Write(data)
	l.mu.Unlock()
}

func (l *speechWsSynthesisListener) OnTextResult(r *tts.SpeechWsSynthesisResponse) {
	fmt.Printf("%s|OnTextResult,sessionId:%s response: %s\n", time.Now().Format("2006-01-02 15:04:05"), l.sessionId, r.ToString())
}

func (l *speechWsSynthesisListener) OnSynthesisFail(r *tts.SpeechWsSynthesisResponse, err error) {
	fmt.Printf("%s|OnSynthesisFail,sessionId:%s response: %s err:%s\n", time.Now().Format("2006-01-02 15:04:05"), l.sessionId, r.ToString(), err.Error())
}
