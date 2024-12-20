package main

import (
	"fmt"
	"github.com/hateeyan/go-mrcp"
	"time"
)

func main() {
	sipClient := &mrcp.SIPClient{
		LocalAddr: "10.9.232.246:5020",
	}

	if err := sipClient.Start(); err != nil {
		panic(err)
	}
	defer sipClient.Close()

	dialog, err := sipClient.Dial("10.9.232.246:8060")
	if err != nil {
		panic(err)
	}
	defer dialog.Close()

	channel, err := dialog.DialMrcpServer(mrcp.ChannelHandler{
		OnMessage: onResponse,
	})
	if err != nil {
		panic(err)
	}

	msg := mrcp.NewMessage(mrcp.MethodRecognize, channel.GetChannelId())
	msg.SetHeader("Recognition-Timeout", "40000")
	msg.SetHeader("No-Input-Timeout", "7000")
	msg.SetHeader("Speech-Incomplete-Timeout", "100")
	msg.SetHeader("Speech-Complete-Timeout", "100")
	msg.SetBody([]byte("session:a4af7ee8-e6ff-4833-8037-5c0bc8b0b692"), "text/uri-list")
	if err := channel.SendMrcpMessage(msg); err != nil {
		panic(err)
	}

	time.Sleep(10 * time.Second)
}

func onResponse(msg mrcp.Message) {
	fmt.Println(msg.GetMessageType(), msg.GetName(), string(msg.GetBody()))
}
