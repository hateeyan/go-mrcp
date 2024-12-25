package mrcp

import (
	"reflect"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	type args struct {
		msg []byte
	}
	tests := []struct {
		name    string
		args    args
		want    Message
		wantErr bool
	}{
		{
			name: "headers",
			args: args{msg: []byte("MRCP/2.0 387 RECOGNIZE 2\r\nChannel-Identifier: 24208d6b89a1403f@speechrecog\r\nContent-Type: text/uri-list\r\nCancel-If-Queue: false\r\nRecognition-Timeout: 40000\r\nConfidence-Threshold:0.5\r\nSensitivity-Level: 5.0\r\nStart-Input-Timers: false\r\nNo-Input-Timeout: 7000\r\nSpeech-Incomplete-Timeout: 100\r\nSpeech-Complete-Timeout: 100\r\nContent-Length: 44\r\n\r\nsession:a4af7ee8-e6ff-4833-8037-5c0bc8b0b692")},
			want: Message{
				messageType: MessageTypeRequest,
				length:      387,
				name:        MethodRecognize,
				requestId:   2,
				headers: map[string]string{
					"Channel-Identifier":        "24208d6b89a1403f@speechrecog",
					"Content-Type":              "text/uri-list",
					"Cancel-If-Queue":           "false",
					"Recognition-Timeout":       "40000",
					"Confidence-Threshold":      "0.5",
					"Sensitivity-Level":         "5.0",
					"Start-Input-Timers":        "false",
					"No-Input-Timeout":          "7000",
					"Speech-Incomplete-Timeout": "100",
					"Speech-Complete-Timeout":   "100",
					"Content-Length":            "44",
				},
				body: []byte("session:a4af7ee8-e6ff-4833-8037-5c0bc8b0b692"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Unmarshal(tt.args.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Unmarshal() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessage_Marshal(t *testing.T) {
	type fields struct {
		messageType MessageType
		Length      int
		Method      string
		RequestId   uint32
		Headers     map[string]string
		Body        []byte
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "recognize",
			fields: fields{
				messageType: MessageTypeRequest,
				Length:      387,
				Method:      MethodRecognize,
				RequestId:   2,
				Headers: map[string]string{
					"Channel-Identifier":        "24208d6b89a1403f@speechrecog",
					"Content-Type":              "text/uri-list",
					"Cancel-If-Queue":           "false",
					"Recognition-Timeout":       "40000",
					"Confidence-Threshold":      "0.5",
					"Sensitivity-Level":         "5.0",
					"Start-Input-Timers":        "false",
					"No-Input-Timeout":          "7000",
					"Speech-Incomplete-Timeout": "100",
					"Speech-Complete-Timeout":   "100",
					"Content-Length":            "44",
				},
				Body: []byte("session:a4af7ee8-e6ff-4833-8037-5c0bc8b0b692"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Message{
				messageType: tt.fields.messageType,
				length:      tt.fields.Length,
				name:        tt.fields.Method,
				requestId:   tt.fields.RequestId,
				headers:     tt.fields.Headers,
				body:        tt.fields.Body,
			}
			data := m.Marshal()
			got, err := Unmarshal(data)
			if err != nil {
				t.Error(err)
				return
			}
			if !reflect.DeepEqual(got, m) {
				t.Errorf("Unmarshal() = %v, want %v", got, m)
			}
		})
	}
}

func TestMessage_parseStartLine(t *testing.T) {
	type args struct {
		line []byte
	}
	tests := []struct {
		name    string
		args    args
		want    Message
		wantErr bool
	}{
		{
			name: "request",
			args: args{line: []byte("MRCP/2.0 387 RECOGNIZE 2")},
			want: Message{
				messageType: MessageTypeRequest,
				length:      387,
				name:        "RECOGNIZE",
				requestId:   2,
			},
			wantErr: false,
		},
		{
			name: "response",
			args: args{line: []byte("MRCP/2.0 112 1 200 COMPLETE")},
			want: Message{
				messageType:  MessageTypeResponse,
				length:       112,
				requestId:    1,
				requestState: "COMPLETE",
				statusCode:   200,
			},
			wantErr: false,
		},
		{
			name: "event",
			args: args{line: []byte("MRCP/2.0 1078 RECOGNITION-COMPLETE 2 COMPLETE")},
			want: Message{
				messageType:  MessageTypeEvent,
				length:       1078,
				name:         "RECOGNITION-COMPLETE",
				requestId:    2,
				requestState: "COMPLETE",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m Message
			if err := m.parseStartLine(tt.args.line); (err != nil) != tt.wantErr {
				t.Errorf("parseStartLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(m, tt.want) {
				t.Errorf("parseStartLine() got = %v, want %v", m, tt.want)
			}
		})
	}
}

func TestMessage_GetCompletionCause(t *testing.T) {
	type fields struct {
		headers map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		want   CompletionCause
	}{
		{
			name: "000",
			fields: fields{
				headers: map[string]string{
					HeaderCompletionCause: "000 success",
				},
			},
			want: 0,
		},
		{
			name: "004",
			fields: fields{
				headers: map[string]string{
					HeaderCompletionCause: "004 error",
				},
			},
			want: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{
				headers: tt.fields.headers,
			}
			if got := m.GetCompletionCause(); got != tt.want {
				t.Errorf("GetCompletionCause() = %v, want %v", got, tt.want)
			}
		})
	}
}
