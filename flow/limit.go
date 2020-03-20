// Copyright 2019-2020 NetEase. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package flow

import (
	"sync"
	"sync/atomic"
	"time"
)

type wait struct {
	limit *limit
	wg    sync.WaitGroup
}

// limit record each query.
// Do it as leak-bucket, every query make
//     `limit.remains - 1`
// When req coming, calculate
//     `time.Now() - limit.lastStored ? FillTime`
// Refresh `limit.remains = limit` or make `limit.remains--`.
// Add sync.Mutex for concurrency.
type limit struct {
	stored time.Time
	// Remain qps
	remains int32
	// lastStored time nano
	lastStored int64

	key string

	hold bool

	// for single goroutine
	mu sync.RWMutex
	wg sync.WaitGroup
}

// Key return key word of limit.
func (it *limit) Key() string {
	return it.key
}

// doLimit check, should also singleflight
func (it *limit) doLimit(nowNano int64, limit int32, fillTime int64) int32 {
	it.mu.RLock()
	refresh := nowNano-it.lastStored > fillTime
	it.mu.RUnlock()

	if refresh {
		it.mu.Lock()
		if it.hold { // Only one goroutine hold
			it.mu.Unlock()
			it.wg.Wait()
			return atomic.AddInt32(&it.remains, -1)
		}

		it.wg.Add(1)   // Block
		it.hold = true // Hold
		it.mu.Unlock()

		// Reset
		it.remains = limit
		it.lastStored = nowNano

		it.wg.Done()
		it.hold = false // Release
	}

	// atomic
	return atomic.AddInt32(&it.remains, -1)
}

// newItem create a new limit.
func newItem(nowNano int64, key string, l int32) *limit {
	return &limit{
		stored:     time.Unix(0, nowNano),
		key:        key,
		remains:    l,
		lastStored: nowNano,
	}
}
