package main

import (
	"fmt"
	"github.com/hateeyan/go-mrcp"
	"time"
)

func main() {
	server := mrcp.Server{
		Host:     "10.9.232.246",
		SIPPort:  5060,
		MRCPPort: 1544,
		Handler: mrcp.ServerHandlerFunc{
			OnDialogCreateFunc: onDialogCreate,
		},
	}
	if err := server.Run(); err != nil {
		fmt.Println("failed to start server:", err)
		return
	}
	defer server.Close()
}

func onMessage(c *mrcp.Channel, msg mrcp.Message) {
	fmt.Println("received message:", msg)
	switch msg.GetName() {
	case mrcp.MethodDefineGrammar:
		resp := c.NewResponse(msg, 200, mrcp.RequestStateComplete)
		resp.SetCompletionCause(c.GetResource(), mrcp.RecogCompletionCauseSuccess)
		if err := c.SendMrcpMessage(resp); err != nil {
			fmt.Println("failed to send response:", err)
			return
		}
	case mrcp.MethodRecognize:
		resp := c.NewResponse(msg, 200, mrcp.RequestStateInProgress)
		if err := c.SendMrcpMessage(resp); err != nil {
			fmt.Println("failed to send response:", err)
			return
		}

		time.AfterFunc(2*time.Second, func() {
			event := c.NewEvent("RECOGNITION-COMPLETE", mrcp.RequestStateComplete)
			resp.SetCompletionCause(c.GetResource(), mrcp.RecogCompletionCauseSuccess)
			event.SetBody([]byte(`<?xml version="1.0" encoding="utf-8"?><result><interpretation grammar="session:d696fd26-e3aa-406a-b76b-e04c3ce696ed"><input mode="speech"><noinput/></input></interpretation></result>`), "application/nlsml+xml")
			if err := c.SendMrcpMessage(event); err != nil {
				fmt.Println("failed to send event:", err)
				return
			}
		})
	}
}

func onChannelOpen(channel *mrcp.Channel) mrcp.ChannelHandler {
	return mrcp.ChannelHandlerFunc{
		OnMessageFunc: onMessage,
	}
}

func onMediaOpen(media *mrcp.Media) mrcp.MediaHandler {
	return mrcp.MediaHandlerFunc{
		WriteRTPPacketFunc: onWriteRTP,
	}
}

func onDialogCreate(d *mrcp.DialogServer) (mrcp.DialogHandler, error) {
	return mrcp.DialogHandlerFunc{
		OnMediaOpenFunc:   onMediaOpen,
		OnChannelOpenFunc: onChannelOpen,
	}, nil
}

func onWriteRTP(m *mrcp.Media, rtp []byte) bool {
	fmt.Println("receive rtp:", len(rtp))
	return true
}
