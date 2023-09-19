package k8s

import (
	"context"

	"github.com/gliderlabs/ssh"
	"k8s.io/client-go/tools/remotecommand"
)

// TerminalSession implements TTY support for attach and exec operations.
type TerminalSession struct {
	winCh <-chan ssh.Window
	ctx   context.Context
}

// Init initializes the terminal session and binds it the sess ssh session.
func (t *TerminalSession) Init(sess ssh.Session, ctx context.Context) {
	_, t.winCh, _ = sess.Pty()
	t.ctx = ctx
}

// Next returns new window size if the terminal has been resized.
func (t *TerminalSession) Next() *remotecommand.TerminalSize {
	select {
	case <-t.ctx.Done():
		return nil
	case size := <-t.winCh:
		return &remotecommand.TerminalSize{
			Width:  uint16(size.Width),
			Height: uint16(size.Height),
		}

	}
}
