package hosts

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// Shell dialer interface definition.
type Shell interface {
	SetIO(stdout, stderr io.Writer, stdin io.ReadCloser)
	ChangeWindowSize(win ShellWindowSize) error
	Terminal() error
	OpenTerminal(win ShellWindowSize) error
	Wait() error
	Write(b []byte) error
	Close() error
}

// WebSocketDialer struct for websocket dialer.
type WebSocketDialer struct {
	dialer Shell
	conn   *websocket.Conn
	reader *TerminalReader
}

// NewWebSocketDialer returns new websocket dialer.
func NewWebSocketDialer(conn *websocket.Conn, dialer Shell) *WebSocketDialer {
	d := &WebSocketDialer{
		conn:   conn,
		dialer: dialer,
	}

	r := NewTerminalReader(d.conn)
	r.SetResizeFunction(d.ChangeWindowSize)
	w := NewBinaryWriter(d.conn)

	d.reader = r
	d.dialer.SetIO(w, w, r)
	return d
}

// Close close the WebSocket connection.
func (d *WebSocketDialer) Close() {
	err := d.dialer.Close()
	if err != nil && !errors.Is(err, io.EOF) {
		logrus.Errorf("[websocket-dialer] dialer closed error: %s", err.Error())
	}
}

// Write write bytes to the websocket connection.
func (d *WebSocketDialer) Write(bytes []byte) error {
	return d.dialer.Write(bytes)
}

// Terminal open websocket terminal.
func (d *WebSocketDialer) Terminal(height, width int) error {
	return d.dialer.OpenTerminal(ShellWindowSize{Height: height, Width: width})
}

// ReadMessage read websocket message.
func (d *WebSocketDialer) ReadMessage(ctx context.Context) error {
	return readMessage(ctx, d.conn, d.Close, d.dialer.Wait, d.reader.ClosedCh)
}

// ChangeWindowSize change websocket win size.
func (d *WebSocketDialer) ChangeWindowSize(win ShellWindowSize) {
	err := d.dialer.ChangeWindowSize(win)
	if err != nil {
		logrus.Errorf("[websocket-dialer] failed to change window size: %s", err.Error())
	}
}

// NewBinaryWriter returns new binary writer.
func NewBinaryWriter(con *websocket.Conn) *BinaryWriter {
	return &BinaryWriter{
		conn: con,
	}
}

// BinaryWriter struct for binary writer.
type BinaryWriter struct {
	conn *websocket.Conn
}

// Write binary writer Write implement.
func (s *BinaryWriter) Write(p []byte) (int, error) {
	w, err := s.conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return 0, convert(err)
	}
	defer func() {
		_ = w.Close()
	}()
	var n int
	if len(p) != 0 {
		n, err = w.Write(p)
	}
	return n, err
}

// TerminalReader struct for terminal reader.
type TerminalReader struct {
	conn     *websocket.Conn
	reader   io.Reader
	resize   func(size ShellWindowSize)
	ClosedCh chan bool
}

// NewTerminalReader returns new terminal reader.
func NewTerminalReader(con *websocket.Conn) *TerminalReader {
	return &TerminalReader{
		conn:     con,
		ClosedCh: make(chan bool, 1),
	}
}

// Close used to close terminal reader.
func (t *TerminalReader) Close() error {
	t.ClosedCh <- true
	return nil
}

// SetResizeFunction set terminal reader resize function.
func (t *TerminalReader) SetResizeFunction(resizeFun func(size ShellWindowSize)) {
	t.resize = resizeFun
}

// Read terminal reader Read implement.
func (t *TerminalReader) Read(p []byte) (int, error) {
	var msgType int
	var err error
	for {
		if t.reader == nil {
			msgType, t.reader, err = t.conn.NextReader()
			if err != nil {
				t.reader = nil
				t.ClosedCh <- true
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
					return 0, io.EOF
				}
				return 0, err
			}
		}
		switch msgType {
		case websocket.TextMessage:
			body, e := io.ReadAll(t.reader)
			if e != nil {
				logrus.Errorf("[websocket-dialer] read text message error: %s", e.Error())
				break
			}
			r := ShellWindowSize{}
			if err = json.Unmarshal(body, &r); err != nil {
				logrus.Errorf("[websocket-dialer] failed to convert resize object body: %s", err.Error())
				break
			}
			if r.Width > 0 && r.Height > 0 {
				t.resize(r)
			}
		case websocket.BinaryMessage:
			n, readErr := t.reader.Read(p)
			return n, convert(readErr)
		}
		t.reader = nil
	}
}

// ShellWindowSize struct for window size.
type ShellWindowSize struct {
	Width  int
	Height int
}

func convert(err error) error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*websocket.CloseError); ok && e.Code == websocket.CloseNormalClosure {
		return io.EOF
	}
	return err
}

func readMessage(ctx context.Context, con *websocket.Conn, closeSession func(), wait func() error, stop chan bool) error {
	sessionClosed := make(chan error, 1)
	go func() {
		sessionClosed <- wait()
	}()

	for {
		select {
		case <-ctx.Done():
			closeSession()
			return nil
		case <-sessionClosed:
			closeSession()
			close(sessionClosed)
			_ = con.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "EOF"))
			return nil
		case isStop := <-stop:
			// check stop from client.
			if isStop {
				closeSession()
				close(stop)
				return nil
			}
		}
	}
}
