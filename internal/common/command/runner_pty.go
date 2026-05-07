// Atomic Package Manager
// Copyright (C) 2025 Дмитрий Удалов dmitry@udalov.online
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package command

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
	"sync"

	"apm/internal/common/app"

	"github.com/creack/pty"
)

// executePTY запускает команду через pseudo-tty.
func (r *runner) executePTY(ctx context.Context, args []string, o options) (string, string, error) {
	if len(args) == 0 {
		return "", "", ErrEmptyCommand
	}

	app.Log.Debug("run pty command: ", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	r.setupCmd(cmd, o)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return "", "", err
	}

	if o.ptyRows > 0 && o.ptyCols > 0 {
		if err = pty.Setsize(ptmx, &pty.Winsize{Rows: o.ptyRows, Cols: o.ptyCols}); err != nil {
			_ = ptmx.Close()
			return "", "", err
		}
	}

	var (
		buf bytes.Buffer
		mu  sync.Mutex
	)

	safe := &lockedWriter{buf: &buf, mu: &mu}

	var reader io.Reader = ptmx
	if o.streamHandler != nil {
		reader = io.TeeReader(ptmx, safe)
	}

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		if o.streamHandler != nil {
			o.streamHandler(reader)
			return
		}
		_, _ = io.Copy(safe, ptmx)
	}()

	cmdErr := cmd.Wait()
	_ = ptmx.Close()
	<-handlerDone

	mu.Lock()
	output := buf.String()
	mu.Unlock()

	return output, "", cmdErr
}

type lockedWriter struct {
	buf *bytes.Buffer
	mu  *sync.Mutex
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}
