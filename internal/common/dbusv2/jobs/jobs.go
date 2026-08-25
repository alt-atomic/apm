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

// Package jobs — реестр фоновых задач API v2 и их трансляция в сигналы.
// Реестр хранит только работающие задачи: запись удаляется в момент
// завершения, результат доставляется исключительно сигналом JobFinished —
// клиент подписывается до вызова метода.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
)

// Состояния задачи в сигнале JobFinished; в реестре задача всегда running.
const (
	StateRunning  = "running"
	StateOK       = "ok"
	StateError    = "error"
	StateCanceled = "canceled"
)

type ctxKey struct{}

// WithJob кладёт id задачи в контекст.
func WithJob(ctx context.Context, id uint32) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext достаёт id задачи из контекста.
func FromContext(ctx context.Context) (uint32, bool) {
	id, ok := ctx.Value(ctxKey{}).(uint32)
	return id, ok
}

// Job работающая фоновая задача.
type Job struct {
	id           uint32
	domain       string
	kind         string
	owner        string
	cancellable  bool
	created      time.Time
	cancelAction string
	cancel       context.CancelFunc
}

// State — JSON-представление работающей задачи.
type State struct {
	ID          uint32 `json:"id"`
	Domain      string `json:"domain"`
	Kind        string `json:"kind"`
	State       string `json:"state"`
	Cancellable bool   `json:"cancellable"`
	Created     int64  `json:"created"`
}

// Emitter шлёт сигнал Jobs-интерфейса.
type Emitter func(member string, values ...any)

// Registry реестр работающих задач демона.
type Registry struct {
	mu   sync.Mutex
	ctx  context.Context
	seq  uint32
	jobs map[uint32]*Job
	emit Emitter
}

// NewRegistry создаёт реестр поверх контекста демона.
func NewRegistry(ctx context.Context, emit Emitter) *Registry {
	return &Registry{ctx: ctx, jobs: make(map[uint32]*Job), emit: emit}
}

// Start регистрирует отменяемую задачу и запускает fn в фоне; возвращает id.
func (r *Registry) Start(domain, kind, owner, cancelAction string, fn func(ctx context.Context) (string, error)) uint32 {
	return r.start(domain, kind, owner, cancelAction, true, fn)
}

// StartNoCancel регистрирует неотменяемую задачу: rpm/apt-транзакции
// прерывать нельзя — Cancel для них возвращает ошибку.
func (r *Registry) StartNoCancel(domain, kind, owner string, fn func(ctx context.Context) (string, error)) uint32 {
	return r.start(domain, kind, owner, "", false, fn)
}

func (r *Registry) start(domain, kind, owner, cancelAction string, cancellable bool, fn func(ctx context.Context) (string, error)) uint32 {
	jctx, cancel := context.WithCancel(r.ctx)

	r.mu.Lock()
	r.seq++
	id := r.seq
	r.jobs[id] = &Job{
		id:           id,
		domain:       domain,
		kind:         kind,
		owner:        owner,
		cancellable:  cancellable,
		created:      time.Now(),
		cancelAction: cancelAction,
		cancel:       cancel,
	}
	r.mu.Unlock()

	r.emit("JobStarted", id, domain, kind)

	go func() {
		result, err := fn(WithJob(jctx, id))
		cancel()
		state, message := finalState(err)
		result = finalResult(result, err)

		r.mu.Lock()
		delete(r.jobs, id)
		r.mu.Unlock()

		r.emit("JobFinished", id, state, message, result)
	}()

	return id
}

// finalResult добавляет машиночитаемый тип классифицированной ошибки.
func finalResult(result string, err error) string {
	if apmErr, ok := errors.AsType[apmerr.APMError](err); ok {
		encoded, _ := json.Marshal(struct {
			ErrorType string `json:"errorType"`
		}{ErrorType: apmErr.Type})
		return string(encoded)
	}
	if result == "" {
		return "{}"
	}
	return result
}

// finalState выводит состояние задачи из её ошибки.
func finalState(err error) (state, message string) {
	if err == nil {
		return StateOK, ""
	}
	var apmErr apmerr.APMError
	if errors.Is(err, context.Canceled) || (errors.As(err, &apmErr) && apmErr.Type == apmerr.ErrorTypeCanceled) {
		return StateCanceled, err.Error()
	}
	return StateError, err.Error()
}

// List возвращает снимки работающих задач.
func (r *Registry) List() []State {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]State, 0, len(r.jobs))
	for _, job := range r.jobs {
		out = append(out, job.snapshot())
	}
	return out
}

// Get возвращает снимок работающей задачи по id.
func (r *Registry) Get(id uint32) (State, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.jobs[id]
	if !ok {
		return State{}, apmerr.New(apmerr.ErrorTypeNotFound, fmt.Errorf("job %d is not running", id))
	}
	return job.snapshot(), nil
}

// Cancel отменяет задачу: владелец — свободно, остальные — по authorize.
func (r *Registry) Cancel(id uint32, sender string, authorize func(action string) error) error {
	r.mu.Lock()
	job, ok := r.jobs[id]
	if !ok {
		r.mu.Unlock()
		return apmerr.New(apmerr.ErrorTypeNotFound, fmt.Errorf("job %d is not running", id))
	}
	if !job.cancellable {
		r.mu.Unlock()
		return apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf("job %d cannot be canceled", id))
	}
	owner, action := job.owner, job.cancelAction
	r.mu.Unlock()

	if sender != owner {
		if err := authorize(action); err != nil {
			return err
		}
	}
	job.cancel()
	return nil
}

// snapshot возвращает снимок работающей задачи; вызывается под mutex реестра.
func (j *Job) snapshot() State {
	return State{
		ID:          j.id,
		Domain:      j.domain,
		Kind:        j.kind,
		State:       StateRunning,
		Cancellable: j.cancellable,
		Created:     j.created.Unix(),
	}
}
