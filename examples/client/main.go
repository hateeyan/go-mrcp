package main

import (
	"encoding/binary"
	"fmt"
	"github.com/hateeyan/go-mrcp"
	"github.com/hateeyan/go-mrcp/pkg/pcm"
	"math/rand"
	"os"
)

var (
	pcmf      *os.File
	pcmData   = make([]byte, 320)
	rtpData   = make([]byte, 172)
	sequence  uint16
	timestamp = rand.Uint32()
	ssrc      = rand.Uint32()
	pt        uint8
	responses = make(chan mrcp.Message, 1)
)

func recognize(dialog *mrcp.DialogClient) error {
	channel := dialog.GetChannel()
	msg := channel.NewRequest(mrcp.MethodDefineGrammar)
	msg.SetHeader("Content-Id", "a4af7ee8-e6ff-4833-8037-5c0bc8b0b692")
	msg.SetBody([]byte(`<?xml version="1.0" encoding="utf-8"?><grammar xmlns="http://www.w3.org/2001/06/grammar" xml:lang="en-US" version="1.0" root="service"><rule id="service"></rule></grammar>`), "application/srgs+xml")
	if err := channel.SendMrcpMessage(msg); err != nil {
		return err
	}

	// wait response
	resp := <-responses
	if resp.GetStatusCode() != 200 {
		return fmt.Errorf("failed to define grammar: %v", resp.GetStatusCode())
	}

	// generate RECOGNIZE request
	msg = channel.NewRequest(mrcp.MethodRecognize)
	msg.SetHeader("Recognition-Timeout", "40000")
	msg.SetHeader("No-Input-Timeout", "7000")
	msg.SetHeader("Speech-Incomplete-Timeout", "100")
	msg.SetHeader("Speech-Complete-Timeout", "100")
	msg.SetBody([]byte("session:a4af7ee8-e6ff-4833-8037-5c0bc8b0b692"), "text/uri-list")
	// send RECOGNIZE request
	if err := channel.SendMrcpMessage(msg); err != nil {
		return err
	}

	// wait response
	resp = <-responses
	if resp.GetStatusCode() != 200 {
		return fmt.Errorf("failed to start recognize: %v", resp.GetStatusCode())
	}

	// wait result
	resp = <-responses
	fmt.Printf("completion-cause: %d, body: %s\n", resp.GetCompletionCause(), string(resp.GetBody()))
	return nil
}

func main() {
	client := mrcp.Client{
		Host:    "10.9.232.246",
		SIPPort: 5020,
	}

	// start client
	if err := client.Run(); err != nil {
		fmt.Println(err)
		return
	}
	defer client.Close()

	// connect to mrcp server
	dialog, err := client.Dial(
		"10.9.232.246:8060",
		mrcp.ResourceSpeechrecog,
		mrcp.DialogHandlerFunc{
			OnMediaOpenFunc: func(_ *mrcp.Media) mrcp.MediaHandler {
				return mrcp.MediaHandlerFunc{
					StartTxFunc:       startTx,
					ReadRTPPacketFunc: readRTPPacket,
				}
			},
			OnChannelOpenFunc: func(_ *mrcp.Channel) mrcp.ChannelHandler {
				return mrcp.ChannelHandlerFunc{
					OnMessageFunc: onMessage,
				}
			},
		},
	)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer dialog.Close()

	pcmf, err = os.Open("../../testdata/8k.pcm")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer pcmf.Close()
	if err := recognize(dialog); err != nil {
		fmt.Println(err)
		return
	}
}

func onMessage(c *mrcp.Channel, msg mrcp.Message) {
	responses <- msg
}

func startTx(m *mrcp.Media, codec mrcp.CodecDesc) error {
	pt = uint8(codec.PayloadType)
	return nil
}

func readRTPPacket(m *mrcp.Media) ([]byte, bool) {
	// read pcm from file
	n, err := pcmf.Read(pcmData)
	if err != nil {
		fmt.Println("read frame error:", err)
		return nil, false
	}
	if n != len(pcmData) {
		fmt.Println("no more data")
		return nil, false
	}

	// rtp header
	sequence++
	timestamp += 160
	rtpData[0] = 0x80
	rtpData[1] = pt
	binary.BigEndian.PutUint16(rtpData[2:], sequence)
	binary.BigEndian.PutUint32(rtpData[4:], timestamp)
	binary.BigEndian.PutUint32(rtpData[8:], ssrc)

	// rtp payload
	// pcm to pcmu
	if err := pcm.LinearToMuLaw(pcmData, rtpData[12:]); err != nil {
		fmt.Println("failed to encode to pcmu:", err)
		return nil, false
	}

	return rtpData, true
}
