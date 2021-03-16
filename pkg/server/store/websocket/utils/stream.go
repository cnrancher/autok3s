package websocket

import (
	"context"
	"encoding/json"
	"io"

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
	defer w.Close()
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

func ReadMessage(ctx context.Context, con *websocket.Conn, close func(), write func([]byte), changeSize func(size *WindowSize)) error {
	for {
		select {
		case <-ctx.Done():
			close()
			return nil
		default:
			msgType, data, err := con.ReadMessage()
			if err != nil {
				close()
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
					return nil
				}
				logrus.Errorf("[ssh terminal] read message error: %v", err)
				return err
			}
			switch msgType {
			case websocket.TextMessage:
				r := &WindowSize{}
				if err = json.Unmarshal(data, r); err != nil {
					logrus.Errorf("[ssh terminal] failed to convert resize object body: %v", err)
					continue
				}
				if r.Width > 0 && r.Height > 0 {
					changeSize(r)
				}
			case websocket.BinaryMessage:
				write(data)
			}
		}
	}
}
