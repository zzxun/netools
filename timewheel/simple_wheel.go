package timewheel

import (
	"container/list"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

const (
	defaultPool = 4
)

// Task wrapper a delayed execute function.
type Task struct {
	d       time.Duration // for test
	expired int64
	task    func()

	// for stop this item
	s unsafe.Pointer
	e *list.Element
}

func (t *Task) getSlot() *slot {
	return (*slot)(atomic.LoadPointer(&t.s))
}

func (t *Task) setSlot(b *slot) {
	atomic.StorePointer(&t.s, unsafe.Pointer(b))
}

// Stop task and remove, return true if removed.
func (t *Task) Stop() (stopped bool) {
	for s := t.getSlot(); t != nil; s = t.getSlot() {
		stopped = s.Remove(t)
	}
	return
}

// slot store the key/func
type slot struct {
	expired int64
	items   *list.List

	sync.Mutex
}

func newSlot() *slot {
	return &slot{
		expired: -1,
		items:   list.New(),
	}
}

// Expired atomic read current expired
func (s *slot) Expired() int64 {
	return atomic.LoadInt64(&s.expired)
}

// Expired atomic store expired
func (s *slot) SetExpired(expired int64) bool {
	return atomic.SwapInt64(&s.expired, expired) != expired
}

// Add a task to this slot
func (s *slot) Add(t *Task) {
	s.Lock()
	// put into list
	e := s.items.PushBack(t)
	t.setSlot(s)
	t.e = e
	s.Unlock()
}

func (s *slot) remove(t *Task) bool {
	if t.getSlot() != s {
		return false
	}
	// clear
	s.items.Remove(t.e)
	t.setSlot(nil)
	t.e = nil
	return true
}

// Remove a task
func (s *slot) Remove(t *Task) bool {
	s.Lock()
	defer s.Unlock()
	return s.remove(t)
}

// Flush check every Task in this slot, and re-insert
// it into timewheel.
func (s *slot) Flush(reinsert func(*Task)) {
	s.Lock()
	e := s.items.Front()
	for e != nil {
		next := e.Next()
		t := e.Value.(*Task)
		s.remove(t)
		reinsert(t)
		e = next
	}
	s.Unlock()

	s.SetExpired(-1)
}

// Size of tasks in slot
func (s *slot) Size() int {
	s.Lock()
	defer s.Unlock()
	return s.items.Len()
}

// SimpleWheel a simple multi-level timing wheel
type SimpleWheel struct {
	tick int64 // millisecond
	size int64

	currentTime int64 // millisecond
	interval    int64 // tick * size

	// cycle
	cur   int64
	slots []*slot

	level int

	// next wheel
	overlap unsafe.Pointer

	// pool
	poolSize int
	poolc    chan *Task

	len  int64
	done int64

	donec chan struct{}
}

// NewSimpleTimeWheel return a timimg wheel with size * tick, and executor pool(goroutine) size
func NewSimpleTimeWheel(tick time.Duration, size int64, pool int) *SimpleWheel {
	tickMs := int64(tick / time.Millisecond)
	if tickMs <= 0 {
		panic(fmt.Errorf("tick should >= 1ms"))
	}
	if pool <= 0 {
		pool = defaultPool
	}

	startMs := timeToMs(time.Now())
	return newSimpleWheel(tickMs, size, startMs, 0, pool)
}

func newSimpleWheel(tickMs int64, size int64, startMs int64, lvl int, pool int) *SimpleWheel {
	slots := make([]*slot, size)
	for i := range slots {
		slots[i] = newSlot()
	}
	return &SimpleWheel{
		tick:        tickMs,
		size:        size,
		currentTime: format(startMs, tickMs), // integer multiples
		interval:    tickMs * size,
		slots:       slots,
		donec:       make(chan struct{}, 1),
		level:       lvl,
		poolSize:    pool,
		poolc:       make(chan *Task, pool*10),
	}
}

// Start timing wheel tick
func (w *SimpleWheel) Start() {
	now := time.Now()
	w.currentTime = format(timeToMs(now), w.tick)
	c := time.NewTimer(time.Duration(w.currentTime+w.tick)*time.Millisecond - time.Duration(now.UnixNano()))
	go func() {
		for {
			select {
			case <-w.donec:
				return
			case now = <-c.C: // tick
				t := w.onTick(w, now)
				c.Reset(time.Duration(t+w.tick)*time.Millisecond - time.Duration(now.UnixNano()))
			}
		}
	}()

	for i := 0; i < w.poolSize; i++ {
		go w.run()
	}
}

// Stop timing wheel tick
func (w *SimpleWheel) Stop() {
	close(w.donec)
}

// add insert the task into this time wheel
func (w *SimpleWheel) add(t *Task) bool {
	cut := atomic.LoadInt64(&w.currentTime)
	cur := atomic.LoadInt64(&w.cur)
	if t.expired < cut+w.tick { // in this tick
		return false
	} else if t.expired < cut+w.interval { // in this level
		// calculate index
		lot := (t.expired - cut) / w.tick
		idx := (lot + cur) % w.size
		s := w.slots[idx]
		s.Add(t)
		s.SetExpired(cut + lot*w.tick)
	} else { // next level
		overlap := atomic.LoadPointer(&w.overlap)
		if overlap == nil {
			atomic.CompareAndSwapPointer(&w.overlap, nil, unsafe.Pointer(newSimpleWheel(
				w.interval,
				w.size,
				cut,
				w.level+1,
				w.poolSize,
			)))
		}
		overlap = atomic.LoadPointer(&w.overlap)
		return (*SimpleWheel)(overlap).add(t)
	}
	return true
}

func (w *SimpleWheel) addOrRun(t *Task) {
	if !w.add(t) {
		w.poolc <- t
	}
}

// After add a task to timing wheel with delay time.
func (w *SimpleWheel) After(d time.Duration, task func()) *Task {
	t := &Task{
		d:       d / time.Millisecond,
		expired: timeToMs(time.Now().Add(d)),
		task:    task,
	}
	w.addOrRun(t)
	return t
}

func (w *SimpleWheel) advanceTime(expired int64) {
	if expired >= w.currentTime+w.tick { // mean tick
		w.currentTime = format(expired, w.tick)
	}
}

func (w *SimpleWheel) onTick(entry *SimpleWheel, now time.Time) int64 {
	cut := atomic.LoadInt64(&w.currentTime)
	// not a tick
	if cut+w.tick > timeToMs(now) {
		return w.cur
	}
	// current slot
	atomic.SwapInt64(&w.cur, (atomic.LoadInt64(&w.cur)+1)%w.size)
	// re-add or run
	s := w.slots[atomic.LoadInt64(&w.cur)]
	atomic.SwapInt64(&w.currentTime, timeToMs(now))
	// to next tick
	if s.Size() > 0 {
		w.slots[w.cur] = newSlot()
		// run task
		go s.Flush(entry.addOrRun)
	}

	overlap := atomic.LoadPointer(&w.overlap)
	if overlap != nil {
		(*SimpleWheel)(overlap).onTick(entry, now)
	}

	return w.currentTime
}

func (w *SimpleWheel) run() {
	for {
		select {
		case t := <-w.poolc:
			t.task() // run it
			atomic.AddInt64(&w.done, 1)
		case <-w.donec:
			return
		}
	}
}

// Done atomic get sum of done task.
func (w *SimpleWheel) Done() int64 {
	return atomic.LoadInt64(&w.done)
}

func format(c, a int64) int64 {
	if a <= 0 {
		return c
	}
	return c - c%a
}

func timeToMs(t time.Time) int64 {
	return int64(time.Duration(t.UnixNano()) / time.Millisecond)
}
