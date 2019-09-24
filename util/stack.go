package util

// MaxStackUint64 return max value at O(1)
type MaxStackUint64 struct {
	list []uint64 // as 1 <- 2 <- 3 <- 5 <- 4
	max  []uint64 // as 1 <- 2 <- 3 <- 5 <- 5

	len  int
	size int
}

// Push push a value to stack, refresh max value. Return true if success.
func (s *MaxStackUint64) Push(val uint64) bool {
	if s.len == s.size {
		return false
	}
	if s.len == 0 { // first
		s.list[s.len] = val
		s.max[s.len] = val
	} else {
		s.list[s.len] = val
		if val > s.max[s.len-1] {
			s.max[s.len] = val
		} else {
			s.max[s.len] = s.max[s.len-1]
		}
	}
	// success
	s.len++
	return true
}

// Pop a value from stack, if stack empty, return 0
func (s *MaxStackUint64) Pop() uint64 {
	if s.len == 0 {
		return 0
	}
	c := s.list[s.len-1]
	s.list[s.len-1] = 0
	s.max[s.len-1] = 0
	s.len--
	return c
}

// Max return max value in stack, if stack empty return 0.
func (s *MaxStackUint64) Max() uint64 {
	if s.len == 0 {
		return 0
	}
	return s.max[s.len-1]
}

// Empty return true if stack empty
func (s *MaxStackUint64) Empty() bool {
	return s.len == 0
}

// MaxQueueUint64 return max value at O(2)
type MaxQueueUint64 struct {
	in   MaxStackUint64
	out  MaxStackUint64
	size int
	len  int
}

// NewMaxQueueUint64 return a new MaxQueueUint64
func NewMaxQueueUint64(size int) *MaxQueueUint64 {
	return &MaxQueueUint64{
		in:   MaxStackUint64{make([]uint64, size), make([]uint64, size), 0, size},
		out:  MaxStackUint64{make([]uint64, size), make([]uint64, size), 0, size},
		size: size,
	}
}

// Enqueue a new value, if full, Dequeue one, and then Enqueue
func (q *MaxQueueUint64) Enqueue(val uint64) {
	if q.len == q.size { // max
		q.Dequeue()
	}
	q.in.Push(val)
	q.len++
}

// Dequeue one, return 0 if empty.
func (q *MaxQueueUint64) Dequeue() uint64 {
	if q.len == 0 {
		return 0
	}
	if q.out.Empty() {
		for !q.in.Empty() {
			q.out.Push(q.in.Pop())
		}
	}
	q.len--
	return q.out.Pop()
}

// Max return max value in queue, return 0 if empty.
func (q *MaxQueueUint64) Max() uint64 {
	if q.len == 0 {
		return 0
	}
	if q.in.Max() > q.out.Max() {
		return q.in.Max()
	}
	return q.out.Max()
}
