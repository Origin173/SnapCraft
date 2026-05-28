package minecraft

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/Origin173/SnapCraft/internal/config"
)

// RCONController implements Controller via Minecraft RCON protocol.
type RCONController struct {
	cfg    *config.Config
	conn   net.Conn
	closed bool
}

func NewRCONController(cfg *config.Config) (*RCONController, error) {
	return &RCONController{cfg: cfg}, nil
}

func (c *RCONController) connect(ctx context.Context) error {
	if c.conn != nil {
		return nil
	}
	addr := fmt.Sprintf("%s:%d", c.cfg.Server.Control.RCON.Host, c.cfg.Server.Control.RCON.Port)
	dialer := net.Dialer{Timeout: c.cfg.Server.Control.RCON.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("rcon connect: %w", err)
	}
	c.conn = conn
	if err := c.authenticate(); err != nil {
		conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

const (
	rconAuth         = 3
	rconExecCommand  = 2
	rconAuthResponse = 2
)

type rconPacket struct {
	Size   int32
	ID     int32
	Type   int32
	Body   string
}

func (c *RCONController) authenticate() error {
	resp, err := c.sendPacket(rconAuth, c.cfg.Server.Control.RCON.Password)
	if err != nil {
		return fmt.Errorf("rcon auth: %w", err)
	}
	if resp.ID == -1 {
		return fmt.Errorf("rcon authentication failed")
	}
	return nil
}

func (c *RCONController) command(ctx context.Context, cmd string) (string, error) {
	if err := c.connect(ctx); err != nil {
		return "", err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.cfg.Server.Control.RCON.Timeout)
	}
	c.conn.SetDeadline(deadline)
	resp, err := c.sendPacket(rconExecCommand, cmd)
	if err != nil {
		c.conn.Close()
		c.conn = nil
		return "", err
	}
	return resp.Body, nil
}

func (c *RCONController) sendPacket(packetType int32, body string) (*rconPacket, error) {
	reqID := int32(1)
	if err := writePacket(c.conn, reqID, packetType, body); err != nil {
		return nil, err
	}
	return readPacket(c.conn)
}

func (c *RCONController) SaveOff(ctx context.Context) error {
	_, err := c.command(ctx, "save-off")
	return err
}

func (c *RCONController) SaveAll(ctx context.Context) error {
	_, err := c.command(ctx, "save-all flush")
	return err
}

func (c *RCONController) SaveOn(ctx context.Context) error {
	_, err := c.command(ctx, "save-on")
	return err
}

func (c *RCONController) Say(ctx context.Context, message string) error {
	_, err := c.command(ctx, "say "+message)
	return err
}

func (c *RCONController) Close() error {
	if c.closed || c.conn == nil {
		return nil
	}
	c.closed = true
	return c.conn.Close()
}

func writePacket(w net.Conn, id, packetType int32, body string) error {
	bodyBytes := []byte(body + "\x00\x00")
	size := int32(4 + 4 + len(bodyBytes))
	buf := make([]byte, 4+size)
	putInt32(buf[0:4], size)
	putInt32(buf[4:8], id)
	putInt32(buf[8:12], packetType)
	copy(buf[12:], bodyBytes)
	_, err := w.Write(buf)
	return err
}

func readPacket(r net.Conn) (*rconPacket, error) {
	sizeBuf := make([]byte, 4)
	if _, err := readFull(r, sizeBuf); err != nil {
		return nil, err
	}
	size := getInt32(sizeBuf)
	if size < 10 {
		return nil, fmt.Errorf("invalid rcon packet size: %d", size)
	}
	buf := make([]byte, size)
	if _, err := readFull(r, buf); err != nil {
		return nil, err
	}
	id := getInt32(buf[0:4])
	pType := getInt32(buf[4:8])
	body := string(buf[8 : size-2])
	return &rconPacket{Size: size, ID: id, Type: pType, Body: body}, nil
}

func readFull(r net.Conn, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		got, err := r.Read(buf[n:])
		if err != nil {
			return n, err
		}
		n += got
	}
	return n, nil
}

func putInt32(b []byte, v int32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func getInt32(b []byte) int32 {
	return int32(b[0]) | int32(b[1])<<8 | int32(b[2])<<16 | int32(b[3])<<24
}
