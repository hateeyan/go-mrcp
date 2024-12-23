package mrcp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
)

const (
	MethodRecognize             = "RECOGNIZE"
	MethodSetParams             = "SET-PARAMS"
	MethodGetParams             = "GET-PARAMS"
	MethodDefineGrammar         = "DEFINE-GRAMMAR"
	MethodInterpret             = "INTERPRET"
	MethodGetResult             = "GET-RESULT"
	MethodStartInputTimers      = "START-INPUT-TIMERS"
	MethodStop                  = "STOP"
	MethodStartPhraseEnrollment = "START-PHRASE-ENROLLMENT"
	MethodEnrollmentRollback    = "ENROLLMENT-ROLLBACK"
	MethodEndPhraseEnrollment   = "END-PHRASE-ENROLLMENT"
	MethodModifyPhrase          = "MODIFY-PHRASE"
	MethodDeletePhrase          = "DELETE-PHRASE"
	MethodSpeak                 = "SPEAK"
	MethodPause                 = "PAUSE"
	MethodResume                = "RESUME"
	MethodBargeInOccurred       = "BARGE-IN-OCCURRED"
	MethodControl               = "CONTROL"
	MethodDefineLexicon         = "DEFINE-LEXICON"
)

const (
	HeaderContentType   = "Content-Type"
	HeaderContentLength = "Content-Length"
)

const (
	RequestStateComplete   = "COMPLETE"
	RequestStateInProgress = "IN-PROGRESS"
	RequestStatePending    = "PENDING"
)

type MessageType uint8

const (
	MessageTypeRequest MessageType = 1 + iota
	MessageTypeResponse
	MessageTypeEvent
)

type Message struct {
	messageType MessageType
	length      int
	// name method-name / event-name
	name         string
	requestId    uint32
	requestState string
	statusCode   int
	headers      map[string]string
	body         []byte
}

func NewMessage(method, channelId string) Message {
	return Message{
		name:      method,
		requestId: 1,
		headers: map[string]string{
			"Channel-Identifier": channelId,
		},
	}
}

func Unmarshal(msg []byte) (Message, error) {
	r := bufio.NewReader(bytes.NewReader(msg))

	// start line
	var m Message
	line, _, err := r.ReadLine()
	if err != nil {
		return Message{}, err
	}
	if err := m.parseStartLine(line); err != nil {
		return Message{}, err
	}

	// headers
	if err := m.parseHeaders(r); err != nil {
		return Message{}, err
	}

	// body
	m.body = make([]byte, r.Buffered())
	if _, err := r.Read(m.body); err != nil {
		return Message{}, err
	}

	return m, nil
}

func (m *Message) Marshal() []byte {
	buf1 := bytes.NewBuffer(make([]byte, 0, 256))
	buf1.WriteString("\r\n")
	for k, v := range m.headers {
		buf1.WriteString(k)
		buf1.WriteString(": ")
		buf1.WriteString(v)
		buf1.WriteString("\r\n")
	}
	buf1.WriteString("\r\n")
	buf1.Write(m.body)

	requestId := strconv.FormatUint(uint64(m.requestId), 10)
	n := 11 + len(m.name) + len(requestId) + buf1.Len()
	n += len(strconv.Itoa(n))
	buf := bytes.NewBuffer(make([]byte, 0, n))
	buf.WriteString("MRCP/2.0 ")
	buf.WriteString(strconv.Itoa(n))
	buf.WriteByte(' ')
	buf.WriteString(m.name)
	buf.WriteByte(' ')
	buf.WriteString(requestId)
	buf.Write(buf1.Bytes())

	return buf.Bytes()
}

func (m *Message) parseStartLine(line []byte) error {
	ss := bytes.Split(line, []byte(" "))
	if len(ss) != 4 && len(ss) != 5 {
		return fmt.Errorf("invalid start line: %s", string(line))
	}

	length, err := strconv.Atoi(string(ss[1]))
	if err != nil {
		return err
	}
	m.length = length
	requestId, err := strconv.ParseUint(string(ss[2]), 10, 32)
	if err != nil {
		// request or event
		m.name = string(ss[2])
		requestId, err = strconv.ParseUint(string(ss[3]), 10, 32)
		if err != nil {
			return fmt.Errorf("invalid request id: %v", err)
		}
		m.requestId = uint32(requestId)
		m.messageType = MessageTypeRequest

		if len(ss) == 5 {
			// event
			m.requestState = string(ss[4])
			m.messageType = MessageTypeEvent
		}
	} else {
		// response
		m.requestId = uint32(requestId)
		statusCode, err := strconv.Atoi(string(ss[3]))
		if err != nil {
			return fmt.Errorf("invalid status code: %v", err)
		}
		m.statusCode = statusCode
		m.requestState = string(ss[4])
		m.messageType = MessageTypeResponse
	}

	return nil
}

func (m *Message) parseHeaders(r *bufio.Reader) error {
	m.headers = make(map[string]string)
	for {
		line, _, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if len(line) == 0 {
			break
		}

		i := bytes.IndexByte(line, ':')
		if i == -1 {
			continue
		}

		v := line[i+1:]
		if len(v) > 0 && v[0] == ' ' {
			v = v[1:]
		}
		m.headers[string(line[:i])] = string(v)
	}
	return nil
}

func (m *Message) GetName() string               { return m.name }
func (m *Message) GetMessageType() MessageType   { return m.messageType }
func (m *Message) GetRequestId() uint32          { return m.requestId }
func (m *Message) SetRequestId(requestId uint32) { m.requestId = requestId }
func (m *Message) GetRequestState() string       { return m.requestState }
func (m *Message) GetHeader(key string) string   { return m.headers[key] }
func (m *Message) SetHeader(k, v string)         { m.headers[k] = v }
func (m *Message) GetBody() []byte               { return m.body }

func (m *Message) SetBody(body []byte, contentType string) {
	m.SetHeader(HeaderContentType, contentType)
	m.SetHeader(HeaderContentLength, strconv.Itoa(len(body)))
	m.body = body
}
