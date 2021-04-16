package websocket

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

func NewWriter(con *websocket.Conn) *BinaryWriter {
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
	n, err := w.Write(p)
	return n, err
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

type WindowSize struct {
	Width  int
	Height int
}

type changeSizeFunc func(size *WindowSize)

type TerminalReader struct {
	conn     *websocket.Conn
	reader   io.Reader
	resize   changeSizeFunc
	ClosedCh chan bool
}

func NewReader(con *websocket.Conn) *TerminalReader {
	return &TerminalReader{
		conn:     con,
		ClosedCh: make(chan bool, 1),
	}
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
				logrus.Errorf("read text message error: %v", e)
				break
			}
			r := &WindowSize{}
			if err = json.Unmarshal(body, r); err != nil {
				logrus.Errorf("[terminal] failed to convert resize object body: %v", err)
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

func ReadMessage(ctx context.Context, con *websocket.Conn, closeSession func(), wait func() error, stop chan bool) error {
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
			// check stop from client
			if isStop {
				closeSession()
				close(stop)
				return nil
			}
		}
	}
}
