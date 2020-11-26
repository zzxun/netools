package util

import (
	"container/list"
	"sync"
	"sync/atomic"
)

// FNVHash returns the FNV FNVHash of what.
func FNVHash(key string) uint32 {
	hash := uint32(2166136261)
	const prime32 = uint32(16777619)
	for i := 0; i < len(key); i++ {
		hash *= prime32
		hash ^= uint32(key[i])
	}
	return hash
}

// CMap implements a concurrent map.
type CMap struct {
	shards [shardSize]*Shard
	lru    bool
	ssize  uint32

	sync.Mutex
}

type node struct {
	// TODO: create from pool?
	key, value interface{}
}

// Shard is a cache with random eviction.
type Shard struct {
	// a lru
	items  map[interface{}]*list.Element
	dlist  *list.List
	size   int
	length int32
	evict  bool

	sync.RWMutex
}

// New returns a new cache. When cache full, it will evict randomly.
func New(size int) *CMap {
	return NewCache(size, true)
}

// NewCache returns a new cache. When cache full, it will evict?.
func NewCache(size int, evict bool) *CMap {
	ssize := size / shardSize
	if ssize&(ssize-1) != 0 {
		ssize = NextPow2(ssize)
	}
	if ssize < 64 {
		ssize = 64
	}

	c := &CMap{}

	// Initialize all the shards
	for i := 0; i < shardSize; i++ {
		c.shards[i] = NewShard(ssize, evict)
	}
	c.ssize = shardSize
	return c
}

// NewSmallMap return a small cache with permenent.
func NewSmallMap(size int) *CMap {
	ssize := size / shardSize
	if ssize&(ssize-1) != 0 {
		ssize = NextPow2(ssize)
	}
	if ssize < 4 {
		ssize = 4
	}

	c := &CMap{}

	// Initialize all the shards
	for i := 0; i < shardSize; i++ {
		c.shards[i] = NewShard(ssize, true)
	}
	c.ssize = shardSize
	return c
}

// LRU set lru function enable?
func (c *CMap) LRU(lru bool) {
	c.lru = lru
}

// Add adds a new element to the cache. If the element already exists it is overwritten.
func (c *CMap) Add(key string, el interface{}) int {
	shard := FNVHash(key) & (c.ssize - 1)
	return c.shards[shard].Add(key, el, c.lru)
}

// Get looks up element index under key.
func (c *CMap) Get(key string) (interface{}, bool) {
	shard := FNVHash(key) & (c.ssize - 1)
	return c.shards[shard].Get(key, c.lru)
}

// Remove removes the element indexed with key.
func (c *CMap) Remove(key string) {
	shard := FNVHash(key) & (c.ssize - 1)
	c.shards[shard].Remove(key, c.lru)
}

// Clear all datas
func (c *CMap) Clear() {
	c.Lock()
	defer c.Unlock()
	for _, s := range c.shards {
		s.Clear()
	}
}

// Len returns the number of elements in the cache.
func (c *CMap) Len() int {
	l := 0
	for _, s := range c.shards {
		l += s.Len()
	}
	return l
}

// NewShard returns a new shard with size.
func NewShard(size int, evict bool) *Shard {
	return &Shard{
		items: make(map[interface{}]*list.Element),
		dlist: list.New(),
		size:  size,
		evict: evict,
	}
}

// Add adds element indexed by key into the cache. Any existing element is overwritten
func (s *Shard) Add(key string, el interface{}, lru bool) (evict int) {
	s.Lock()
	if ele, ok := s.items[key]; ok {
		if lru {
			s.dlist.MoveToFront(ele)
		}
		ele.Value.(*node).value = el
		s.Unlock()
		return 0
	}
	n := s.dlist.PushFront(&node{key: key, value: el})
	s.items[key] = n

	atomic.AddInt32(&s.length, 1)

	if s.dlist.Len() > s.size && s.evict {
		for i := 10; i > 0; i++ {
			ent := s.dlist.Back()
			if ent == nil {
				s.Unlock()
				return
			}
			s.dlist.Remove(ent)
			delete(s.items, ent.Value.(*node).key)
			atomic.AddInt32(&s.length, -1)
			evict++
		}
	}
	s.Unlock()
	return
}

// Remove removes the element indexed by key from the cache.
func (s *Shard) Remove(key string, lru bool) {
	s.Lock()
	if ele, ok := s.items[key]; ok {
		delete(s.items, key)
		s.dlist.Remove(ele)
		atomic.AddInt32(&s.length, -1)
	}
	s.Unlock()
}

// Evict removes a random 3 element from the cache.
func (s *Shard) Evict() {
	s.Lock()
	ent := s.dlist.Back()
	if ent == nil {
		s.Unlock()
		return
	}
	s.dlist.Remove(ent)
	delete(s.items, ent.Value.(*node).key)
	atomic.AddInt32(&s.length, -1)
	s.Unlock()
}

// Get looks up the element indexed under key.
func (s *Shard) Get(key string, lru bool) (interface{}, bool) {
	s.RLock()
	el, found := s.items[key]
	if found {
		val := el.Value.(*node).value
		s.RUnlock()
		if lru {
			s.Lock()
			s.dlist.MoveToFront(el)
			s.Unlock()
		}
		return val, found
	}
	s.RUnlock()
	return nil, found
}

// Clear all datas
func (s *Shard) Clear() {
	s.Lock()
	defer s.Unlock()
	s.length = 0
	s.dlist = nil
	s.items = nil
}

// Len returns the current length of the cache.
func (s *Shard) Len() int {
	return int(atomic.LoadInt32(&s.length))
}

const (
	shardSize = 256
)
