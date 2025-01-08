package mrcp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
)

type ChannelId struct {
	Id       string
	Resource Resource
}

func parseChannelId(raw string) ChannelId {
	i := strings.IndexByte(raw, '@')
	if i == -1 {
		return ChannelId{}
	}
	return ChannelId{Id: raw[:i], Resource: Resource(raw[i+1:])}
}

func (c ChannelId) String() string { return c.Id + "@" + string(c.Resource) }

type connectionHandler interface {
	// OnMessage on receive mrcp message
	OnMessage(c *connection, msg Message)
}

type connectionHandlerFunc struct {
	OnMessageFunc func(c *connection, msg Message)
}

func (h connectionHandlerFunc) OnMessage(c *connection, msg Message) {
	if h.OnMessageFunc != nil {
		h.OnMessageFunc(c, msg)
	}
}

type connection struct {
	conn    net.Conn
	handler connectionHandler
	logger  *slog.Logger
}

func (s *Server) accept(conn net.Conn, handler connectionHandler) {
	c := &connection{conn: conn, handler: handler, logger: s.Logger}
	go c.startReadMessage()
}

func (c *connection) startReadMessage() {
	r := bufio.NewReader(c.conn)
	buf := make([]byte, 1024)
	for {
		peek, err := r.Peek(20)
		if err != nil {
			if err != io.EOF && !errors.Is(err, net.ErrClosed) {
				c.logger.Error("unable to read from mrcp server", "error", err)
			}
			break
		}

		if !bytes.HasPrefix(peek, []byte("MRCP/2.0 ")) {
			c.logger.Error("invalid mrcp start line", "line", string(peek))
			break
		}

		i := bytes.IndexByte(peek[9:], ' ')
		if i == -1 {
			// TODO: peek more data instead of return
			c.logger.Error("failed to get message length", "line", string(peek[9:]))
			break
		}
		length, err := strconv.Atoi(string(peek[9 : 9+i]))
		if err != nil {
			c.logger.Error("failed to parse message length", "error", err)
			break
		}

		if len(buf) < length {
			buf = make([]byte, length)
		}
		if _, err := io.ReadFull(r, buf[:length]); err != nil {
			c.logger.Error("failed to read message", "error", err)
			break
		}

		msg, err := Unmarshal(buf[:length])
		if err != nil {
			c.logger.Error("failed to parse message", "error", err)
			continue
		}

		if c.handler != nil {
			c.handler.OnMessage(c, msg)
		}
	}
}

func (c *connection) writeMessage(msg Message) error {
	_, err := c.conn.Write(msg.Marshal())
	return err
}

func (c *connection) Close() error {
	if c == nil {
		return nil
	}
	return c.conn.Close()
}

type ChannelHandler interface {
	// OnMessage on receive mrcp message
	OnMessage(c *Channel, msg Message)
}

type ChannelHandlerFunc struct {
	// OnMessageFunc mrcp response
	OnMessageFunc func(c *Channel, msg Message)
}

func (h ChannelHandlerFunc) OnMessage(c *Channel, msg Message) {
	if h.OnMessageFunc != nil {
		h.OnMessageFunc(c, msg)
	}
}

type Channel struct {
	id        ChannelId
	requestId uint32
	conn      *connection
	handler   ChannelHandler
	closed    bool
	logger    *slog.Logger
}

func (d *DialogClient) dialMRCPServer() error {
	if d.channel != nil {
		return nil
	}

	if d.rdesc.ControlDesc.ChannelId.Id == "" {
		return fmt.Errorf("invalid channel identifier: %s", d.rdesc.ControlDesc.ChannelId)
	}

	conn, err := net.Dial("tcp", d.rdesc.ControlDesc.Host+":"+strconv.Itoa(d.rdesc.ControlDesc.Port))
	if err != nil {
		return err
	}
	d.channel = &Channel{
		id:     d.rdesc.ControlDesc.ChannelId,
		logger: d.logger,
	}
	if d.handler != nil {
		d.channel.handler = d.handler.OnChannelOpen(d.channel)
	}
	d.channel.conn = &connection{
		conn: conn,
		handler: connectionHandlerFunc{
			OnMessageFunc: func(_ *connection, msg Message) { d.channel.onMessage(msg) },
		},
		logger: d.logger,
	}
	go d.channel.conn.startReadMessage()
	return nil
}

func (c *Channel) NewRequest(method string) Message {
	c.requestId++
	return Message{
		messageType: MessageTypeRequest,
		name:        method,
		requestId:   c.requestId,
		headers: map[string]string{
			HeaderChannelIdentifier: c.id.String(),
		},
	}
}

func (c *Channel) NewResponse(msg Message, statusCode int, requestState string) Message {
	c.requestId = msg.requestId
	return Message{
		messageType:  MessageTypeResponse,
		requestId:    msg.requestId,
		requestState: requestState,
		statusCode:   statusCode,
		headers: map[string]string{
			HeaderChannelIdentifier: c.id.String(),
		},
	}
}

func (c *Channel) NewEvent(event, requestState string) Message {
	return Message{
		messageType:  MessageTypeEvent,
		name:         event,
		requestId:    c.requestId,
		requestState: requestState,
		headers: map[string]string{
			HeaderChannelIdentifier: c.id.String(),
		},
	}
}

// TODO: check Channel inused
func (c *Channel) SendMrcpMessage(msg Message) error {
	return c.conn.writeMessage(msg)
}

func (c *Channel) bind(conn *connection) {
	if c.conn != nil {
		return
	}
	c.conn = conn
}

func (c *Channel) bound() bool {
	return c.conn != nil
}

func (c *Channel) onMessage(msg Message) {
	if c.handler != nil {
		c.handler.OnMessage(c, msg)
	}
}

func (c *Channel) GetChannelId() ChannelId   { return c.id }
func (c *Channel) SetChannelId(id ChannelId) { c.id = id }
func (c *Channel) GetResource() Resource     { return c.id.Resource }

func (c *Channel) Close() error {
	if c == nil || c.closed {
		return nil
	}
	c.closed = true
	return c.conn.Close()
}
