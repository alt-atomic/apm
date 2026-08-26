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
// Завершённая задача остаётся в реестре ещё retention: клиент узнаёт id из
// ответа метода и мог получить JobFinished раньше него, поэтому результат
// обязан быть доступен и через Get, а не только сигналом.
package jobs

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"slices"
	"sync"
	"time"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/app"
)

// Состояния задачи: running до завершения, дальше одно из терминальных.
const (
	StateRunning  = "running"
	StateOK       = "ok"
	StateError    = "error"
	StateCanceled = "canceled"
)

const (
	retention   = 20 * time.Minute
	emptyResult = "{}"
)

type ctxKey struct{}

// WithJob кладёт id задачи в контекст.
func WithJob(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext достаёт id задачи из контекста.
func FromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(ctxKey{}).(string)
	return id, ok
}

// Job фоновая задача демона: работающая или недавно завершённая.
type Job struct {
	id           string
	seq          uint64
	domain       string
	kind         string
	owner        string
	cancellable  bool
	created      time.Time
	cancelAction string
	cancel       context.CancelFunc

	state     string
	errorType string
	message   string
	result    string
	finished  time.Time
}

// State — JSON-представление задачи.
type State struct {
	ID          string          `json:"id"`
	Domain      string          `json:"domain"`
	Kind        string          `json:"kind"`
	State       string          `json:"state"`
	Cancellable bool            `json:"cancellable"`
	Created     int64           `json:"created"`
	Finished    int64           `json:"finished,omitempty"`
	ErrorType   string          `json:"errorType,omitempty"`
	Message     string          `json:"message,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
}

// Emitter шлёт сигнал Jobs-интерфейса.
type Emitter func(member string, values ...any)

// Registry реестр задач демона.
type Registry struct {
	mu        sync.Mutex
	ctx       context.Context
	prefix    string
	seq       uint64
	jobs      map[string]*Job
	emit      Emitter
	retention time.Duration
}

// NewRegistry создаёт реестр поверх контекста демона. prefix уникален для
// запуска демона, поэтому id задачи не повторяется после перезапуска.
func NewRegistry(ctx context.Context, prefix string, emit Emitter) *Registry {
	if prefix == "" {
		prefix = "job"
	}
	return &Registry{
		ctx:       ctx,
		prefix:    prefix,
		jobs:      make(map[string]*Job),
		emit:      emit,
		retention: retention,
	}
}

// Start регистрирует отменяемую задачу и запускает fn в фоне; возвращает id.
func (r *Registry) Start(domain, kind, owner, cancelAction string, fn func(ctx context.Context) (string, error)) string {
	return r.start(domain, kind, owner, cancelAction, true, fn)
}

// StartNoCancel регистрирует неотменяемую задачу: rpm/apt-транзакции
// прерывать нельзя — Cancel для них возвращает ошибку.
func (r *Registry) StartNoCancel(domain, kind, owner string, fn func(ctx context.Context) (string, error)) string {
	return r.start(domain, kind, owner, "", false, fn)
}

func (r *Registry) start(domain, kind, owner, cancelAction string, cancellable bool, fn func(ctx context.Context) (string, error)) string {
	jctx, cancel := context.WithCancel(r.ctx)

	r.mu.Lock()
	r.prune(time.Now())
	r.seq++
	job := &Job{
		id:           fmt.Sprintf("%s-%d", r.prefix, r.seq),
		seq:          r.seq,
		domain:       domain,
		kind:         kind,
		owner:        owner,
		cancellable:  cancellable,
		created:      time.Now(),
		cancelAction: cancelAction,
		cancel:       cancel,
		state:        StateRunning,
	}
	r.jobs[job.id] = job
	r.mu.Unlock()

	r.emit("JobStarted", job.id, domain, kind)

	go func() {
		result, err := run(jctx, job.id, fn)
		cancel()

		state, errorType, message := finalState(err)
		r.finish(job, state, errorType, message, result)

		if result == "" {
			result = emptyResult
		}
		r.emit("JobFinished", job.id, state, errorType, message, result)
	}()

	return job.id
}

// run выполняет задачу, превращая панику в ошибку: упавший модуль не должен
// уносить демон и оставлять задачу вечно работающей.
func run(ctx context.Context, id string, fn func(ctx context.Context) (string, error)) (result string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			app.Log.Errorf("job %s panicked: %v\n%s", id, recovered, debug.Stack())
			result, err = "", fmt.Errorf("internal error: %v", recovered)
		}
	}()
	return fn(WithJob(ctx, id))
}

// finish переводит задачу в терминальное состояние, запись живёт ещё retention.
func (r *Registry) finish(job *Job, state, errorType, message, result string) {
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()
	job.state = state
	job.errorType = errorType
	job.message = message
	job.result = result
	job.finished = now
	r.prune(now)
}

// finalState выводит состояние, машинный тип и текст из ошибки задачи.
func finalState(err error) (state, errorType, message string) {
	if err == nil {
		return StateOK, "", ""
	}
	apmErr, classified := errors.AsType[apmerr.APMError](err)
	if errors.Is(err, context.Canceled) || (classified && apmErr.Type == apmerr.ErrorTypeCanceled) {
		return StateCanceled, apmerr.ErrorTypeCanceled, err.Error()
	}
	if classified {
		return StateError, apmErr.Type, err.Error()
	}
	return StateError, "", err.Error()
}

// List возвращает снимки задач в порядке запуска.
func (r *Registry) List() []State {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prune(time.Now())

	jobs := make([]*Job, 0, len(r.jobs))
	for _, job := range r.jobs {
		jobs = append(jobs, job)
	}
	slices.SortFunc(jobs, func(a, b *Job) int { return cmp.Compare(a.seq, b.seq) })

	out := make([]State, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, job.snapshot())
	}
	return out
}

// Get возвращает снимок задачи: работающей или завершённой в пределах retention.
func (r *Registry) Get(id string) (State, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prune(time.Now())

	job, ok := r.jobs[id]
	if !ok {
		return State{}, apmerr.New(apmerr.ErrorTypeNotFound, fmt.Errorf("job %s is unknown", id))
	}
	return job.snapshot(), nil
}

// Cancel отменяет задачу: владелец — свободно, остальные — по authorize.
func (r *Registry) Cancel(id string, sender string, authorize func(action string) error) error {
	r.mu.Lock()
	job, ok := r.jobs[id]
	if !ok {
		r.mu.Unlock()
		return apmerr.New(apmerr.ErrorTypeNotFound, fmt.Errorf("job %s is unknown", id))
	}
	if job.state != StateRunning {
		r.mu.Unlock()
		return apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf("job %s is already finished", id))
	}
	if !job.cancellable {
		r.mu.Unlock()
		return apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf("job %s cannot be canceled", id))
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

// prune удаляет завершённые задачи старше retention.
func (r *Registry) prune(now time.Time) {
	for id, job := range r.jobs {
		if job.state != StateRunning && now.Sub(job.finished) > r.retention {
			delete(r.jobs, id)
		}
	}
}

// snapshot возвращает снимок задачи; вызывается под mutex реестра.
func (j *Job) snapshot() State {
	state := State{
		ID:          j.id,
		Domain:      j.domain,
		Kind:        j.kind,
		State:       j.state,
		Cancellable: j.cancellable,
		Created:     j.created.Unix(),
	}
	if j.state == StateRunning {
		return state
	}

	state.Finished = j.finished.Unix()
	state.ErrorType = j.errorType
	state.Message = j.message
	if j.result != "" {
		state.Result = json.RawMessage(j.result)
	}
	return state
}
