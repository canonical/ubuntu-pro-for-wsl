package worker

import (
	"context"
	"sync"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/task"
)

// taskQueue is a queue that allows pushing and pulling tasks from a FIFO queue,
// with the particularity that duplicated elements will be removed in favour of
// the latest one.
//
// Pulling from an empty queue will wait until it is no longer empty.
//
// The queue is thread-safe, but there is no guarantee that awaiting pulls will
// be served in order of arrival.
type taskQueue struct {
	mu   sync.RWMutex
	wait chan struct{}
	data []task.Task
}

// newWaitChannel creates a channel to notify waiters of new tasks.
func newWaitChannel() chan struct{} {
	// Tests has shown that's possible to have writers reaching the channel before any reader, thus the notification may never arrive.
	// The amount of writers is a best-guess at this moment and may be adjusted.
	return make(chan struct{}, 4)
}

func newTaskQueue() *taskQueue {
	return &taskQueue{
		mu:   sync.RWMutex{},
		wait: newWaitChannel(),
		data: make([]task.Task, 0),
	}
}

// Load replaces the existing data with the one in "newData".
func (q *taskQueue) Load(newData []task.Task) {
	q.mu.Lock()
	defer q.mu.Unlock()

	close(q.wait)
	q.wait = newWaitChannel()

	q.data = newData
}

// Absorb takes all entries from another queue. The other queue is left empty.
func (q *taskQueue) Absorb(other *taskQueue) {
	other.mu.Lock()
	defer other.mu.Unlock()

	q.mu.Lock()
	defer q.mu.Unlock()

	transferedData := other.data

	close(other.wait)
	other.wait = newWaitChannel()
	other.data = make([]task.Task, 0)

	close(q.wait)
	q.wait = newWaitChannel()
	q.data = append(q.data, transferedData...)
}

// Len returns the number of tasks in the queue.
func (q *taskQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return len(q.data)
}

// Data returns a copy of all the queued tasks.
func (q *taskQueue) Data() []task.Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return append([]task.Task{}, q.data...)
}

// Push adds a task to the queue. Any existing equivalent tasks are removed.
func (q *taskQueue) Push(t task.Task) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Remove copies of this task
	q.data = removeIf(q.data, func(queued task.Task) bool { return task.Is(t, queued) })

	// Append task
	q.data = append(q.data, t)

	// Notify waiters if there are any
	select {
	case q.wait <- struct{}{}:
	default:
	}
}

// Push adds a task to the queue unless an equivalent task is queued already.
// Useful for re-submitting failed tasks.
func (q *taskQueue) PushIfNew(t task.Task) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if this task exists already
	for _, queued := range q.data {
		if task.Is(queued, t) {
			return
		}
	}

	// Append task
	q.data = append(q.data, t)

	// Notify waiters if there are any
	select {
	case q.wait <- struct{}{}:
	default:
	}
}

// Contains returns true if a task equivalent to "t" is queued.
func (q *taskQueue) Contains(t task.Task) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()

	for _, queued := range q.data {
		if task.Is(queued, t) {
			return true
		}
	}

	return false
}

// Remove erases all tasks that are equivalent to "t".
func (q *taskQueue) Remove(t task.Task) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.data = removeIf(q.data, func(queued task.Task) bool { return task.Is(t, queued) })
}

// Pull pops the first task in the queue. If the queue is empty, this function
// blocks until a task is Pushed, Loaded or Absorved.
//
// Concurrent pulls are safe but the order in which they are served in is
// indeterminate.
func (q *taskQueue) Pull(ctx context.Context) task.Task {
	// Avoid races if the context is cancelled already
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	for {
		if task, ok := q.tryPopFront(); ok {
			return task
		}

		q.mu.RLock()
		// This is mostly to appease the race detector
		wait := q.wait
		q.mu.RUnlock()

		select {
		case <-ctx.Done():
			return nil
		case <-wait:
			// ↑
			// | Race here: another goroutine could "steal" the
			// | only entry in the queue. Or an empty Load could
			// | leave an empty "data" behind.
			// ↓
			if task, ok := q.tryPopFront(); ok {
				return task
			}
			// Solution to race: just try again
		}
	}
}

// tryPopFront is a helper function not to be used outside. Equivalent to Pull but without
// waiting. It returns false if the queue is empty.
func (q *taskQueue) tryPopFront() (task.Task, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.data) == 0 {
		return nil, false
	}

	r := q.data[0]
	q.data = q.data[1:]
	return r, true
}

// removeIf removes all elements that satisfy the predicate from the array.
func removeIf(array []task.Task, predicate func(task.Task) bool) []task.Task {
	// Accepts or rejects every entry of the slice, pushing accepted
	// entries to the end of the accepted region.
	//
	// Ordering is preserved for the accepted entries, but not for
	// the rejected ones.
	//
	// A half-processed slice would look like this:
	//
	// |--- accepted ----|----- rejected ----|...unprocessed...|
	// 0                 j                   i                 end
	//
	j := 0
	for i := range array {
		if predicate(array[i]) {
			// Rejected
			continue
		}

		if i == j {
			j++
			continue
		}

		array[i], array[j] = array[j], array[i]
		j++
	}

	return array[0:j]
}
