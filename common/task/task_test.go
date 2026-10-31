package task

import (
	"log"
	"sync"
	"testing"
	"time"
)

func TestTask(t *testing.T) {
	ts := Task{Execute: func() error {
		log.Println("q")
		return nil
	}, Interval: time.Second}
	ts.Start(false)
}

// 在 Execute 内部 Close 再 Start（面板修改拉取间隔时 nodeInfoMonitor 就是这样重启自身的），
// 旧循环必须退出，不能和新循环并行执行。
func TestTaskRestartFromExecute(t *testing.T) {
	var (
		mu        sync.Mutex
		calls     int
		active    int
		overlap   bool
		restarted bool
	)
	ts := &Task{Interval: time.Millisecond}
	ts.Execute = func() error {
		mu.Lock()
		calls++
		if !restarted {
			restarted = true
			mu.Unlock()
			ts.Close()
			_ = ts.Start(false)
			return nil
		}
		active++
		if active > 1 {
			overlap = true
		}
		mu.Unlock()
		time.Sleep(3 * time.Millisecond)
		mu.Lock()
		active--
		mu.Unlock()
		return nil
	}
	_ = ts.Start(true)
	time.Sleep(100 * time.Millisecond)
	ts.Close()

	mu.Lock()
	defer mu.Unlock()
	if calls < 2 {
		t.Fatalf("task did not keep running after restart, calls = %d", calls)
	}
	if overlap {
		t.Fatal("old loop kept running after restart: Execute ran concurrently")
	}
}

// Close 之后不应再执行
func TestTaskCloseStops(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	ts := &Task{Interval: time.Millisecond, Execute: func() error {
		mu.Lock()
		calls++
		mu.Unlock()
		return nil
	}}
	_ = ts.Start(false)
	time.Sleep(20 * time.Millisecond)
	ts.Close()
	time.Sleep(5 * time.Millisecond)
	mu.Lock()
	n := calls
	mu.Unlock()
	time.Sleep(20 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if calls != n {
		t.Fatalf("task kept running after Close: %d -> %d", n, calls)
	}
}
