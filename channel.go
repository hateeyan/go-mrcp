package mrcp

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"net"
	"strconv"
)

type ChannelHandler struct {
	// OnMessage mrcp response
	OnMessage func(c *Channel, msg Message)
}

type Channel struct {
	id        string
	requestId uint32
	conn      net.Conn
	handler   ChannelHandler
	logger    *slog.Logger
}

func (d *DialogClient) dialMrcpServer(handler ChannelHandler) error {
	if d.channel != nil {
		return nil
	}

	conn, err := net.Dial("tcp", d.rdesc.ControlDesc.Host+":"+strconv.Itoa(d.rdesc.ControlDesc.Port))
	if err != nil {
		return err
	}
	d.channel = &Channel{
		id:      d.rdesc.ControlDesc.Channel,
		conn:    conn,
		handler: handler,
		logger:  d.sc.Logger,
	}
	go d.channel.startReadResponse()
	return nil
}

func (c *Channel) NewMessage(method string) Message {
	c.requestId++
	return Message{
		name:      method,
		requestId: c.requestId,
		headers: map[string]string{
			"Channel-Identifier": c.id,
		},
	}
}

func (c *Channel) SendMrcpMessage(msg Message) error {
	_, err := c.conn.Write(msg.Marshal())
	return err
}

func (c *Channel) startReadResponse() {
	r := bufio.NewReader(c.conn)
	buf := make([]byte, 1024)
	for {
		peek, err := r.Peek(20)
		if err != nil {
			if err != io.EOF {
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

		if c.handler.OnMessage != nil {
			c.handler.OnMessage(c, msg)
		}
	}
}

func (c *Channel) GetChannelId() string { return c.id }

func (c *Channel) Close() error {
	if c == nil {
		return nil
	}
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return err
		}
	}
	return nil
}
