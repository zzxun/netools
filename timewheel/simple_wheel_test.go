package timewheel

import (
	"fmt"
	"github.com/zzxun/netools/util"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestTimingWheel_AfterFunc(t *testing.T) {
	st := NewSimpleTimeWheel(time.Millisecond, 20, 8)
	st.Start()

	defer st.Stop()

	durations := []time.Duration{
		1 * time.Millisecond,
		5 * time.Millisecond,
		10 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
	}
	for _, d := range durations {
		t.Run("", func(t *testing.T) {
			exitC := make(chan time.Time)
			//t.Log(1, st)
			star := st.currentTime
			start := time.Now()
			st.After(d, func() {
				exitC <- time.Now()
			})
			//t.Log(2, st)

			got := (<-exitC).Truncate(time.Millisecond)
			star2 := st.currentTime
			min := start.Add(d).Truncate(time.Millisecond)

			err := 5 * time.Millisecond
			if got.Before(min) || got.After(min.Add(err)) {
				t.Errorf("NewTimer(%s) want [%d, %d], got [%d, %d, %d]", d, timeToMs(min), timeToMs(min.Add(err)), star, star2, timeToMs(got))
			}
		})
	}
}

func TestTimingWheel_AfterFunc2(t *testing.T) {
	st := NewSimpleTimeWheel(time.Second, 60, 10)
	st.Start()

	defer st.Stop()

	durations := []time.Duration{
		1 * time.Millisecond,
		5 * time.Millisecond,
		10 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
		3 * time.Second,
	}
	for _, d := range durations {
		t.Run("", func(t *testing.T) {
			exitC := make(chan time.Time)
			//t.Log(1, st)
			star := st.currentTime
			start := time.Now()
			st.After(d, func() {
				exitC <- time.Now()
			})
			//t.Log(2, st)

			got := (<-exitC).Truncate(time.Second)
			star2 := st.currentTime
			min := start.Add(d).Truncate(time.Second)

			err := 5 * time.Millisecond
			if got.Before(min) || got.After(min.Add(err)) {
				t.Errorf("NewTimer(%s) want [%d, %d], got [%d, %d, %d]", d, timeToMs(min), timeToMs(min.Add(err)), star, star2, timeToMs(got))
			}
		})
	}
}

func TestASimpleWheel_After(t *testing.T) {
	wheel := NewSimpleTimeWheel(time.Second/2, 60, 4)
	wheel.Start()

	defer wheel.Stop()

	var proc int64
	c := util.New(1000000)

	go func() {
		select {
		case <-time.After(time.Second * 5):
			fmt.Println(c.Len(), atomic.LoadInt64(&proc))
		}
	}()

	for i := 0; i < 200000; i++ {
		key := strconv.FormatInt(int64(i), 10)
		c.Add(key, struct{}{})
		wheel.After(5*time.Second, func() {
			c.Remove(key)
			atomic.AddInt64(&proc, 1)
		})
	}

	select {
	case <-time.After(time.Second * 6):
		fmt.Println(c.Len(), atomic.LoadInt64(&proc))
	}

}

func BenchmarkSimpleWheel_After(b *testing.B) {
	wheel := NewSimpleTimeWheel(time.Second, 60, 4)
	wheel.Start()

	defer wheel.Stop()

	var proc int64

	c := util.New(1000000)
	for i := 0; i < b.N; i++ {
		key := strconv.FormatInt(int64(i), 10)
		c.Add(key, struct{}{})
		wheel.After(time.Second/2, func() {
			c.Remove(key)
			atomic.AddInt64(&proc, 1)
		})
	}

	if atomic.LoadInt64(&proc) < int64(b.N)/2 {
		b.Errorf("should del all")
	}
}

func Test_After(t *testing.T) {

	var proc int64

	c := util.New(1000000)

	caps := []int{
		1,
		100,
		1000,
		10000,
		100000,
		1000000,
	}
	for _, ca := range caps {
		atomic.StoreInt64(&proc, 0)
		for i := 0; i < ca; i++ {
			key := strconv.FormatInt(int64(i), 10)
			c.Add(key, struct{}{})
			time.AfterFunc(time.Second/2, func() {
				c.Remove(key)
				atomic.AddInt64(&proc, 1)
			})
		}
		select {
		case <-time.After(501 * time.Millisecond):
			t.Log(atomic.LoadInt64(&proc), ca)
		}

		if atomic.LoadInt64(&proc) < int64(ca) {
			t.Error("should del all", proc, ca)
		}
	}

}
