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
		messageType  MessageType
		Length       int
		Name         string
		RequestId    uint32
		RequestState string
		StatusCode   int
		Headers      map[string]string
		Body         []byte
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "request",
			fields: fields{
				messageType: MessageTypeRequest,
				Length:      387,
				Name:        MethodRecognize,
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
		{
			name: "response",
			fields: fields{
				messageType:  MessageTypeResponse,
				Length:       112,
				RequestId:    1,
				RequestState: "COMPLETE",
				StatusCode:   200,
				Headers: map[string]string{
					"Channel-Identifier": "b2587e873c604dcf@speechrecog",
					"Completion-Cause":   "000 success",
				},
				Body: []byte{},
			},
		},
		{
			name: "event",
			fields: fields{
				messageType:  MessageTypeEvent,
				Length:       1088,
				Name:         "RECOGNITION-COMPLETE",
				RequestId:    2,
				RequestState: "COMPLETE",
				Headers: map[string]string{
					"Channel-Identifier": "b2587e873c604dcf@speechrecog",
					"Completion-Cause":   "000 success",
					"Content-Type":       "application/nlsml+xml",
					"Content-Length":     "900",
				},
				Body: []byte("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<result>\n  <interpretation grammar=\"session:d696fd26-e3aa-406a-b76b-e04c3ce696ed\">\n    <instance>\n      <nlpSpeech>PGF1ZGlvIHNyYz0iaHR0cHM6Ly9zdG9yYWdlLmpkLmNvbS9vcGVuLmppbWkucmVzb3VyY2UuZXh0LzEvYW5zd2VyLzIwMjQxMi8zMzZjMTg3ZWFkNTZkMTYxYmExMmYxM2YwYWRjNDE5ZC5tcDMiLz4=;encoding=base64;vid=1697775369063;cid=2322568;mid=70cddf6d-ba82-4db9-8ba4-121dd9ad9ac5;sid=67c95bb8b9ca425b94a82af6f64d911c@1_1321298939134640133_4857096;endpointip=10.29.0.87;endpointport=6021;holdttsch=1;revokebarparmid=70cddf6d-ba82-4db9-8ba4-121dd9ad9ac5;nlpmsgstime=1735093326816;nlpterm=1;nlpdiagseq=1;bargeinstatus=0;bargeinmode=percent;bargeinvpercent=30</nlpSpeech>\n      <command/>\n      <business>{}</business>\n      <mid>70cddf6d-ba82-4db9-8ba4-121dd9ad9ac5</mid>\n      <input/>\n    </instance>\n    <input mode=\"speech\">\n      <noinput/>\n    </input>\n  </interpretation>\n</result>\n"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Message{
				messageType:  tt.fields.messageType,
				length:       tt.fields.Length,
				name:         tt.fields.Name,
				requestId:    tt.fields.RequestId,
				requestState: tt.fields.RequestState,
				statusCode:   tt.fields.StatusCode,
				headers:      tt.fields.Headers,
				body:         tt.fields.Body,
			}
			data := m.Marshal()
			got, err := Unmarshal(data)
			if err != nil {
				t.Error(err)
				return
			}
			if !reflect.DeepEqual(got, m) {
				t.Errorf("Unmarshal() = \n%v, want \n%v", got, m)
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
