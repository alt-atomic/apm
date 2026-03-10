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
	"io"
)

// dividerWriter разделяет поток вывода по строке-разделителю.
type dividerWriter struct {
	mainWriter   io.Writer
	outputWriter io.Writer
	divider      []byte
	found        bool
	buf          []byte
}

func newDividerWriter(mainWriter, outputWriter io.Writer, divider string) *dividerWriter {
	return &dividerWriter{
		mainWriter:   mainWriter,
		outputWriter: outputWriter,
		divider:      []byte(divider),
	}
}

func (w *dividerWriter) Write(p []byte) (int, error) {
	n := len(p)

	if w.found {
		_, err := w.outputWriter.Write(p)
		return n, err
	}

	w.buf = append(w.buf, p...)

	idx := bytes.Index(w.buf, w.divider)
	if idx == -1 {
		safe := len(w.buf) - len(w.divider) + 1
		if safe > 0 {
			if _, err := w.mainWriter.Write(w.buf[:safe]); err != nil {
				return n, err
			}
			w.buf = w.buf[safe:]
		}
		return n, nil
	}

	w.found = true

	if idx > 0 {
		if _, err := w.mainWriter.Write(w.buf[:idx]); err != nil {
			return n, err
		}
	}

	after := w.buf[idx+len(w.divider):]
	if len(after) > 0 && after[0] == '\n' {
		after = after[1:]
	}

	if len(after) > 0 {
		if _, err := w.outputWriter.Write(after); err != nil {
			return n, err
		}
	}

	w.buf = nil
	return n, nil
}

// flush записывает оставшийся буфер в mainWriter.
func (w *dividerWriter) flush() error {
	if len(w.buf) > 0 && !w.found {
		_, err := w.mainWriter.Write(w.buf)
		w.buf = nil
		return err
	}
	return nil
}
