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
	"sync/atomic"
	"testing"
	"time"
)

func TestNewFlow(t *testing.T) {
	flow := NewFlowControl(1024, time.Second)

	r := flow.Do("test", 1)
	if r < 0 {
		t.Fatal(r)
	}
	r = flow.Do("test", 1)
	if r >= 0 {
		t.Fatal(r)
	}
	r = flow.Do("test", 1)
	if r >= 0 {
		t.Fatal(r)
	}

	<-time.After(time.Second)
	r = flow.Do("test", 1)
	if r < 0 {
		t.Fatal(r)
	}
	r = flow.Do("test", 1)
	if r >= 0 {
		t.Fatal(r)
	}
	r = flow.Do("test", 1)
	if r >= 0 {
		t.Fatal(r)
	}
}

func BenchmarkNewFlowControl(b *testing.B) {
	flow := NewFlowControl(1024, time.Hour)
	count := int64(0)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r := flow.Do("test", 1)
			if r >= 0 {
				atomic.AddInt64(&count, 1)
			}
		}
	})

	if atomic.LoadInt64(&count) != 1 {
		b.Fatal(atomic.LoadInt64(&count))
	}
}
