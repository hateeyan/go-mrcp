package main

import (
	"encoding/binary"
	"fmt"
	"github.com/hateeyan/go-mrcp"
	"github.com/hateeyan/go-mrcp/pkg/pcm"
	"math/rand"
	"os"
	"time"
)

var (
	pcmf      *os.File
	pcmData   = make([]byte, 320)
	rtpData   = make([]byte, 172)
	sequence  uint16
	timestamp = rand.Uint32()
	ssrc      = rand.Uint32()
	pt        uint8
)

func recognize(dialog *mrcp.DialogClient) error {
	// generate RECOGNIZE request
	msg := dialog.GetChannel().NewMessage(mrcp.MethodRecognize)
	msg.SetHeader("Recognition-Timeout", "40000")
	msg.SetHeader("No-Input-Timeout", "7000")
	msg.SetHeader("Speech-Incomplete-Timeout", "100")
	msg.SetHeader("Speech-Complete-Timeout", "100")
	msg.SetBody([]byte("session:a4af7ee8-e6ff-4833-8037-5c0bc8b0b692"), "text/uri-list")
	// send RECOGNIZE request
	if err := dialog.GetChannel().SendMrcpMessage(msg); err != nil {
		return err
	}
	return nil
}

func speak(dialog *mrcp.DialogClient) error {
	// generate SPEAK request
	msg := dialog.GetChannel().NewMessage(mrcp.MethodSpeak)
	msg.SetHeader("Voice-Name", "someone")
	msg.SetBody([]byte(`<?xml version="1.0"?><speak version="1.0" xml:lang="en-Us" xmlns="http://www.w3.org/2001/10/synthesis">Welcome to go-mrcp</speak>`), "application/ssml+xml")
	// send SPEAK request
	if err := dialog.GetChannel().SendMrcpMessage(msg); err != nil {
		return err
	}
	return nil
}

func main() {
	sipClient := &mrcp.SIPClient{
		LocalAddr: "10.9.232.246:5020",
	}

	// run sip client
	if err := sipClient.Run(); err != nil {
		fmt.Println(err)
		return
	}
	defer sipClient.Close()

	// connect to mrcp server
	dialog, err := sipClient.Dial(
		"10.9.232.246:8060",
		mrcp.ResourceSpeechrecog,
		mrcp.MediaHandler{
			StartTx:        startTx,
			ReadRTPPacket:  readRTPPacket,
			WriteRTPPacket: writeRTPPacket,
		},
		mrcp.ChannelHandler{
			OnMessage: onResponse,
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

	time.Sleep(10 * time.Second)
}

func onResponse(c *mrcp.Channel, msg mrcp.Message) {
	fmt.Println(msg.GetMessageType(), msg.GetName(), string(msg.GetBody()))
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
	if err := pcm.LinearToULaw(pcmData, rtpData[12:]); err != nil {
		fmt.Println("failed to encode to pcmu:", err)
		return nil, false
	}

	return rtpData, true
}

func writeRTPPacket(m *mrcp.Media, rtp []byte) bool {
	fmt.Println("received rtp packet:", len(rtp))
	return true
}
