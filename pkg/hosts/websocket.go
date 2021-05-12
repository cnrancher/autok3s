package hosts

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type Dialer interface {
	SetIO(stdout, stderr io.Writer, stdin io.ReadCloser)
	SetWindowSize(height, weight int)
	ChangeWindowSize(win *WindowSize) error
	OpenTerminal() error
	Wait() error
	Write(b []byte) error
	Close() error
}

type WebSocketDialer struct {
	dialer Dialer
	conn   *websocket.Conn
	reader *TerminalReader
}

func NewWebSocketDialer(conn *websocket.Conn, dialer Dialer) *WebSocketDialer {
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

// SetDefaultSize set dialer's default window size.
func (d *WebSocketDialer) SetDefaultSize(height, weight int) {
	d.dialer.SetWindowSize(height, weight)
}

// Write write bytes to the websocket connection.
func (d *WebSocketDialer) Write(bytes []byte) error {
	return d.dialer.Write(bytes)
}

// Terminal open websocket terminal.
func (d *WebSocketDialer) Terminal() error {
	return d.dialer.OpenTerminal()
}

// ReadMessage read websocket message.
func (d *WebSocketDialer) ReadMessage(ctx context.Context) error {
	return readMessage(ctx, d.conn, d.Close, d.dialer.Wait, d.reader.ClosedCh)
}

// ChangeWindowSize change websocket win size.
func (d *WebSocketDialer) ChangeWindowSize(win *WindowSize) {
	err := d.dialer.ChangeWindowSize(win)
	if err != nil {
		logrus.Errorf("[websocket-dialer] failed to change window size: %s", err.Error())
	}
}

func NewBinaryWriter(con *websocket.Conn) *BinaryWriter {
	return &BinaryWriter{
		conn: con,
	}
}

type BinaryWriter struct {
	conn *websocket.Conn
}

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

type TerminalReader struct {
	conn     *websocket.Conn
	reader   io.Reader
	resize   changeSizeFunc
	ClosedCh chan bool
}

func NewTerminalReader(con *websocket.Conn) *TerminalReader {
	return &TerminalReader{
		conn:     con,
		ClosedCh: make(chan bool, 1),
	}
}

func (t *TerminalReader) Close() error {
	t.ClosedCh <- true
	return nil
}

func (t *TerminalReader) SetResizeFunction(resizeFun func(size *WindowSize)) {
	t.resize = resizeFun
}

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
			body, e := ioutil.ReadAll(t.reader)
			if e != nil {
				logrus.Errorf("[websocket-dialer] read text message error: %s", e.Error())
				break
			}
			r := &WindowSize{}
			if err = json.Unmarshal(body, r); err != nil {
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

type WindowSize struct {
	Width  int
	Height int
}

type changeSizeFunc func(size *WindowSize)

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
