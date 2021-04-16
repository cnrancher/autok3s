package hosts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"

	"github.com/cnrancher/autok3s/pkg/types"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

const (
	SSHKind    = "ssh"
	DockerKind = "docker"
	PtyKind    = "pty"
)

type WebSocketDialer struct {
	kind         string
	sshDialer    *SSHDialer
	dockerDialer *DockerDialer
	ptyDialer    *PtyDialer

	conn    *websocket.Conn
	session *ssh.Session
	reader  *TerminalReader

	err error
}

func NewWebSocketDialer(kind string, n *types.Node, conn *websocket.Conn, cmd *exec.Cmd) (*WebSocketDialer, error) {
	d := &WebSocketDialer{
		kind: kind,
		conn: conn,
	}

	r := NewTerminalReader(d.conn)
	r.SetResizeFunction(d.ChangeWindowSize)
	w := NewBinaryWriter(d.conn)

	d.reader = r

	switch kind {
	case SSHKind:
		sshDialer, err := NewSSHDialer(n, true)
		if err != nil {
			return nil, err
		}

		sshDialer.SetStdio(w, w, r)

		session, err := sshDialer.conn.NewSession()
		if err != nil {
			return nil, err
		}

		d.session = session
		d.sshDialer = sshDialer
	case DockerKind:
		dockerDialer, err := NewDockerDialer(n)
		if err != nil {
			return nil, err
		}

		dockerDialer.SetStdio(w, w, r)
		d.dockerDialer = dockerDialer
	case PtyKind:
		ptyDialer, err := NewPtyDialer(cmd)
		if err != nil {
			return nil, err
		}

		ptyDialer.SetStdio(w, w, r)

		d.ptyDialer = ptyDialer
	default:
		return nil, fmt.Errorf("[websocket-dialer] dialer type is invalid")
	}

	return d, nil
}

// Close close the WebSocket connection.
func (d *WebSocketDialer) Close() {
	var err error

	if d.sshDialer != nil {
		err = d.sshDialer.Close()
	}

	if d.dockerDialer != nil {
		err = d.dockerDialer.Close()
	}

	if d.ptyDialer != nil {
		err = d.ptyDialer.Close()
	}

	if err != nil && !errors.Is(err, io.EOF) {
		logrus.Errorf("[websocket-dialer] dialer closed error: %s", err.Error())
	}
}

// SetDefaultSize set dialer's default win size.
func (d *WebSocketDialer) SetDefaultSize(height, weight int) *WebSocketDialer {
	switch d.kind {
	case SSHKind:
		d.sshDialer = d.sshDialer.SetDefaultSize(height, weight)
	case DockerKind:
		d.dockerDialer = d.dockerDialer.SetDefaultSize(height, weight)
	case PtyKind:
		d.ptyDialer = d.ptyDialer.SetDefaultSize(height, weight)
	}
	return d
}

// Write write bytes to the websocket connection.
func (d *WebSocketDialer) Write(bytes []byte) error {
	var err error

	switch d.kind {
	case PtyKind:
		_, err = d.ptyDialer.conn.Write(bytes)
	}

	return err
}

// Terminal open websocket terminal.
func (d *WebSocketDialer) Terminal() error {
	switch d.kind {
	case SSHKind:
		return d.sshDialer.WebSocketTerminal(d.session)
	case DockerKind:
		return d.dockerDialer.WebSocketTerminal()
	case PtyKind:
		return d.ptyDialer.WebSocketTerminal()
	}
	return nil
}

// ReadMessage read websocket message.
func (d *WebSocketDialer) ReadMessage(ctx context.Context) error {
	switch d.kind {
	case SSHKind:
		return readMessage(ctx, d.conn, d.Close, d.session.Wait, d.reader.ClosedCh)
	case DockerKind:
		return readMessage(ctx, d.conn, d.Close, d.dockerDialer.TerminalWait, d.reader.ClosedCh)
	case PtyKind:
		return readMessage(ctx, d.conn, d.Close, d.ptyDialer.cmd.Wait, d.reader.ClosedCh)
	}
	return nil
}

// ChangeWindowSize change websocket win size.
func (d *WebSocketDialer) ChangeWindowSize(win *WindowSize) {
	var err error
	switch d.kind {
	case SSHKind:
		err = d.session.WindowChange(win.Height, win.Width)
	//case DockerKind:
	//	err = d.dockerDialer.ResizeTtyTo(d.dockerDialer.ctx, uint(win.Height), uint(win.Width))
	case PtyKind:
		err = d.ptyDialer.ChangeSize(win)
	}

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
