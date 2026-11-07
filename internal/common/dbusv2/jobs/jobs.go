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

const (
	StateQueued   = "queued"
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

// Job фоновая задача демона: ожидающая, работающая или недавно завершённая.
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
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	prefix    string
	seq       uint64
	jobs      map[string]*Job
	queue     *queue
	emit      Emitter
	retention time.Duration
}

// NewRegistry создаёт реестр поверх контекста демона. prefix уникален для
// запуска демона, поэтому id задачи не повторяется после перезапуска.
func NewRegistry(ctx context.Context, prefix string, emit Emitter) *Registry {
	if prefix == "" {
		prefix = "job"
	}
	registryCtx, cancel := context.WithCancel(ctx)
	return &Registry{
		ctx:       registryCtx,
		cancel:    cancel,
		prefix:    prefix,
		jobs:      make(map[string]*Job),
		queue:     newQueue(),
		emit:      emit,
		retention: retention,
	}
}

// Start регистрирует отменяемую задачу и ставит fn на фоновое выполнение.
func (r *Registry) Start(resource Resource, domain, kind, owner, cancelAction string, fn func(ctx context.Context) (string, error)) string {
	return r.start(resource, domain, kind, owner, cancelAction, true, fn)
}

// StartNoCancel регистрирует неотменяемую задачу: пакетную транзакцию
// прерывать нельзя — Cancel для неё возвращает ошибку.
func (r *Registry) StartNoCancel(resource Resource, domain, kind, owner string, fn func(ctx context.Context) (string, error)) string {
	return r.start(resource, domain, kind, owner, "", false, fn)
}

func (r *Registry) start(resource Resource, domain, kind, owner, cancelAction string, cancellable bool, fn func(ctx context.Context) (string, error)) string {
	jctx, cancel := context.WithCancel(r.ctx)
	state := StateRunning
	if resource != ResourceNone {
		state = StateQueued
	}

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
		state:        state,
	}
	r.jobs[job.id] = job
	if err := r.ctx.Err(); err != nil {
		r.mu.Unlock()
		r.complete(job, "", err)
		return job.id
	}
	r.wg.Add(1)
	ticket := r.queue.Enter(resource)
	r.mu.Unlock()

	go func() {
		defer r.wg.Done()
		defer r.queue.Leave(ticket)
		if err := r.queue.Wait(jctx, ticket); err != nil {
			r.complete(job, "", err)
			return
		}
		r.markRunning(job)
		r.emit("JobStarted", job.id, domain, kind)

		runCtx := jctx
		if !cancellable {
			runCtx = context.WithoutCancel(jctx)
		}
		result, err := run(runCtx, job.id, fn)
		r.complete(job, result, err)
	}()

	return job.id
}

// Shutdown перестаёт принимать задачи, отменяет ожидающие и отменяемые задачи
// и ждёт завершения уже запущенных неотменяемых операций. Таймаута нет, rpm-транзакцию прерывать нельзя.
func (r *Registry) Shutdown() {
	r.mu.Lock()
	r.cancel()
	queued, running := r.activeLocked()
	r.mu.Unlock()

	app.Log.Debugf("jobs shutdown: %d queued canceled, waiting for %d running", queued, running)
	r.wg.Wait()
	app.Log.Debug("jobs shutdown: all jobs finished")
}

// activeLocked считает незавершённые задачи; вызывается под mutex реестра.
func (r *Registry) activeLocked() (queued, running int) {
	for _, job := range r.jobs {
		switch job.state {
		case StateQueued:
			queued++
		case StateRunning:
			running++
		}
	}
	return queued, running
}

// markRunning переводит получившую ресурс задачу из очереди в выполнение.
func (r *Registry) markRunning(job *Job) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job.state = StateRunning
}

// complete фиксирует результат и отправляет терминальный сигнал.
func (r *Registry) complete(job *Job, result string, err error) {
	job.cancel()
	state, errorType, message := finalState(err)
	if result == "" {
		result = emptyResult
	}
	r.finish(job, state, errorType, message, result)
	r.emit("JobFinished", job.id, state, errorType, message, result)
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
	if !job.active() {
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
		if !job.active() && now.Sub(job.finished) > r.retention {
			delete(r.jobs, id)
		}
	}
}

// active — задача ещё не завершена: ждёт ресурс или выполняется.
func (j *Job) active() bool {
	return j.state == StateQueued || j.state == StateRunning
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
	if j.active() {
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
