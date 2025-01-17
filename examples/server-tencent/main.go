package main

import (
	"fmt"
	"github.com/hateeyan/go-mrcp"
)

const (
	secretId  = ""
	secretKey = ""
	appId     = 0
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

func onDialogCreate(d *mrcp.DialogServer) (mrcp.DialogHandler, error) {
	switch d.GetResource() {
	case mrcp.ResourceSpeechsynth:
		return newSynthesis()
	case mrcp.ResourceSpeechrecog:
		return newRecognition()
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", d.GetResource())
	}
}
