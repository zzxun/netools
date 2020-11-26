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
	"runtime"
	"sync"
	"time"

	"github.com/zzxun/netools/timewheel"
	"github.com/zzxun/netools/util"
)

// Control of flow to limit requests in a fixed time interval.
type Control interface {
	// Do control qps, return a remain qps limit
	Do(key string, limit int32) int32
}

type flowControl struct {
	// config
	interval int64
	// singleflight
	single map[string]*wait
	cmap   *util.CMap

	remainTLL time.Duration
	cleaner   *timewheel.SimpleWheel

	mu sync.RWMutex
}

// NewFlowControl return a new Control.
func NewFlowControl(size int, interval time.Duration) Control {
	f := &flowControl{
		interval:  int64(interval),
		remainTLL: 15 * time.Minute,
		single:    make(map[string]*wait),
		cleaner:   timewheel.NewSimpleTimeWheel(time.Second/2, 60, 1),
		cmap:      util.NewSmallMap(size),
	}
	f.cleaner.Start()
	runtime.SetFinalizer(f.cleaner, (*timewheel.SimpleWheel).Stop)
	return f
}

// NewFlowControl return a new Control.
func NewFlowControl2(size int,
	interval time.Duration,
	cmap *util.CMap,
	cleaner *timewheel.SimpleWheel,
) Control {
	f := &flowControl{
		interval:  int64(interval),
		remainTLL: 15 * time.Minute,
		single:    make(map[string]*wait),
		cleaner:   cleaner,
		cmap:      cmap,
	}
	if f.cmap == nil {
		f.cmap = util.NewSmallMap(size)
	}
	if f.cleaner == nil {
		f.cleaner = timewheel.NewSimpleTimeWheel(time.Second/2, 60, 1)
		f.cleaner.Start()
		runtime.SetFinalizer(f.cleaner, (*timewheel.SimpleWheel).Stop)
	}
	return f
}

// GetLimit of this key
func (f *flowControl) Do(key string, li int32) int32 {
	now := time.Now().UnixNano()
	it, ok := f.cmap.Get(key)
	if ok {
		return it.(*limit).doLimit(now, li, f.interval)
	}
	f.mu.Lock()
	if w, ok := f.single[key]; ok {
		f.mu.Unlock()
		w.wg.Wait()
		return w.limit.doLimit(now, li, f.interval)
	}

	w := new(wait)
	w.wg.Add(1)
	f.single[key] = w
	f.mu.Unlock()

	w.limit = newItem(now, key, li)
	f.cmap.Add(key, w.limit)
	f.cleaner.After(
		f.remainTLL, func() {
			f.cmap.Remove(key)
		},
	)
	w.wg.Done()

	f.mu.Lock()
	delete(f.single, key)
	f.mu.Unlock()
	return w.limit.doLimit(now, li, f.interval)
}
