package main

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/hateeyan/go-mrcp"
	"github.com/hateeyan/go-mrcp/pkg/pcm"
	"github.com/tencentcloud/tencentcloud-speech-sdk-go/asr"
	"github.com/tencentcloud/tencentcloud-speech-sdk-go/common"
	"strconv"
	"time"
)

type speechRecognitionListener struct {
	sessionId string
	pcm       []byte
	next      int
	recog     *asr.SpeechRecognizer
	channel   *mrcp.Channel
}

func newRecognition() (*speechRecognitionListener, error) {
	listener := &speechRecognitionListener{
		sessionId: uuid.NewString(),
		pcm:       make([]byte, 5*320),
	}
	credential := common.NewCredential(secretId, secretKey)
	listener.recog = asr.NewSpeechRecognizer(strconv.FormatInt(appId, 10), credential, "8k_zh", listener)
	listener.recog.VoiceID = listener.sessionId
	listener.recog.VoiceFormat = asr.AudioFormatPCM

	return listener, nil
}

func (l *speechRecognitionListener) onWriteRTPPacket(m *mrcp.Media, rtp []byte) bool {
	if err := pcm.MuLawToLiner(rtp[12:], l.pcm[l.next*320:]); err != nil {
		fmt.Println("failed to decode to pcm:", err)
		return false
	}

	if l.next < 4 {
		l.next++
		return true
	}
	l.next = 0
	if err := l.recog.Write(l.pcm); err != nil {
		fmt.Println("send pcm error:", err)
		return false
	}
	return true
}

func (l *speechRecognitionListener) OnMediaOpen(media *mrcp.Media) mrcp.MediaHandler {
	return mrcp.MediaHandlerFunc{
		WriteRTPPacketFunc: l.onWriteRTPPacket,
	}
}

func (l *speechRecognitionListener) onMessage(c *mrcp.Channel, msg mrcp.Message) {
	switch msg.GetName() {
	case mrcp.MethodDefineGrammar:
		resp := c.NewResponse(msg, 200, mrcp.RequestStateComplete)
		resp.SetCompletionCause(c.GetResource(), mrcp.RecogCompletionCauseSuccess)
		if err := c.SendMrcpMessage(resp); err != nil {
			fmt.Println("failed to send response:", err)
			return
		}
	case mrcp.MethodRecognize:
		if err := l.recog.Start(); err != nil {
			fmt.Printf("%s|recognizer start failed, error: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)

			resp := c.NewResponse(msg, 501, mrcp.RequestStateComplete)
			if err := c.SendMrcpMessage(resp); err != nil {
				fmt.Println("failed to send response:", err)
			}
			return
		}
		resp := c.NewResponse(msg, 200, mrcp.RequestStateInProgress)
		if err := c.SendMrcpMessage(resp); err != nil {
			fmt.Println("failed to send response:", err)
			return
		}
	}
}

func (l *speechRecognitionListener) OnChannelOpen(channel *mrcp.Channel) mrcp.ChannelHandler {
	l.channel = channel
	return mrcp.ChannelHandlerFunc{
		OnMessageFunc: l.onMessage,
	}
}

func (l *speechRecognitionListener) OnClose() {
}

// OnRecognitionStart implementation of SpeechRecognitionListener
func (l *speechRecognitionListener) OnRecognitionStart(response *asr.SpeechRecognitionResponse) {
	fmt.Printf("%s|%s|OnRecognitionStart\n", time.Now().Format("2006-01-02 15:04:05"), response.VoiceID)
}

// OnSentenceBegin implementation of SpeechRecognitionListener
func (l *speechRecognitionListener) OnSentenceBegin(response *asr.SpeechRecognitionResponse) {
	fmt.Printf("%s|%s|OnSentenceBegin: %v\n", time.Now().Format("2006-01-02 15:04:05"), response.VoiceID, response)
}

// OnRecognitionResultChange implementation of SpeechRecognitionListener
func (l *speechRecognitionListener) OnRecognitionResultChange(response *asr.SpeechRecognitionResponse) {
	fmt.Printf("%s|%s|OnRecognitionResultChange: %v\n", time.Now().Format("2006-01-02 15:04:05"), response.VoiceID, response)
}

// OnSentenceEnd implementation of SpeechRecognitionListener
func (l *speechRecognitionListener) OnSentenceEnd(response *asr.SpeechRecognitionResponse) {
	fmt.Printf("%s|%s|OnSentenceEnd: %v\n", time.Now().Format("2006-01-02 15:04:05"), response.VoiceID, response)
	event := l.channel.NewEvent("RECOGNITION-COMPLETE", mrcp.RequestStateComplete)
	event.SetCompletionCause(l.channel.GetResource(), mrcp.RecogCompletionCauseSuccess)
	event.SetBody([]byte(response.Result.VoiceTextStr), "application/nlsml+xml")
	if err := l.channel.SendMrcpMessage(event); err != nil {
		fmt.Println("failed to send event:", err)
		return
	}
}

// OnRecognitionComplete implementation of SpeechRecognitionListener
func (l *speechRecognitionListener) OnRecognitionComplete(response *asr.SpeechRecognitionResponse) {
	fmt.Printf("%s|%s|OnRecognitionComplete\n", time.Now().Format("2006-01-02 15:04:05"), response.VoiceID)
}

// OnFail implementation of SpeechRecognitionListener
func (l *speechRecognitionListener) OnFail(response *asr.SpeechRecognitionResponse, err error) {
	fmt.Printf("%s|%s|OnFail: %v\n", time.Now().Format("2006-01-02 15:04:05"), response.VoiceID, err)
}
