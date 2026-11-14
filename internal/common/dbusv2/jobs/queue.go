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

package jobs

import (
	"context"
	"slices"
	"strings"
	"sync"
)

// Resource объединяет конфликтующие задачи в одну очередь.
type Resource string

const (
	ResourceNone Resource = ""
	// ResourceHost операции над пакетной базой хоста: пакеты, ядро, репозитории.
	ResourceHost Resource = "host"
	// ResourceImage сборка и переключение образа хоста.
	ResourceImage Resource = "image"
)

// ResourceKey создаёт ключ именованного ресурса вида scope:id,
// например distrobox:dev или image:default.
func ResourceKey(scope, id string) Resource {
	return Resource(scope + ":" + strings.TrimSpace(id))
}

type queue struct {
	mu    sync.Mutex
	gates map[Resource]*gate
}

type gate struct {
	holder  *ticket
	waiting []*ticket
}

type ticket struct {
	resource Resource
	ready    chan struct{}
}

func newQueue() *queue {
	return &queue{gates: make(map[Resource]*gate)}
}

// Enter занимает место в очереди ресурса.
func (q *queue) Enter(resource Resource) *ticket {
	t := &ticket{resource: resource, ready: make(chan struct{})}
	if resource == ResourceNone {
		close(t.ready)
		return t
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	g := q.gates[resource]
	if g == nil {
		g = &gate{}
		q.gates[resource] = g
	}
	if g.holder == nil {
		g.holder = t
		close(t.ready)
	} else {
		g.waiting = append(g.waiting, t)
	}
	return t
}

// Wait ждёт получения ресурса или отмены; возвращает ошибку контекста.
func (q *queue) Wait(ctx context.Context, t *ticket) error {
	select {
	case <-t.ready:
	case <-ctx.Done():
	}
	return ctx.Err()
}

// Leave снимает задачу с очереди: держатель передаёт ресурс следующему,
// ожидающий просто удаляется. Пустая очередь освобождает ресурс.
func (q *queue) Leave(t *ticket) {
	if t.resource == ResourceNone {
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	g := q.gates[t.resource]
	if g == nil {
		return
	}
	if g.holder != t {
		if i := slices.Index(g.waiting, t); i >= 0 {
			g.waiting = slices.Delete(g.waiting, i, i+1)
		}
		return
	}
	if len(g.waiting) == 0 {
		delete(q.gates, t.resource)
		return
	}
	next := g.waiting[0]
	g.waiting = slices.Delete(g.waiting, 0, 1)
	g.holder = next
	close(next.ready)
}

// Len число ресурсов с занятой очередью.
func (q *queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.gates)
}
