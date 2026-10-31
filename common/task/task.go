package task

import (
	"sync"
	"time"
)

type Task struct {
	Interval time.Duration
	Execute  func() error
	access   sync.Mutex
	running  bool
	stop     chan struct{}
}

func (t *Task) Start(first bool) error {
	t.access.Lock()
	if t.running {
		t.access.Unlock()
		return nil
	}
	t.running = true
	// 每次启动的循环只使用自己的 stop 和 interval，
	// 这样 Close 后立即重新 Start（包括在 Execute 内部重启自身）时，
	// 旧循环不会挂到新的 stop 上继续运行。
	stop := make(chan struct{})
	t.stop = stop
	interval := t.Interval
	t.access.Unlock()

	go func() {
		if first {
			if err := t.Execute(); err != nil {
				t.stopIfCurrent(stop)
				return
			}
		}

		for {
			select {
			case <-time.After(interval):
			case <-stop:
				return
			}
			select {
			case <-stop:
				return
			default:
			}

			if err := t.Execute(); err != nil {
				t.stopIfCurrent(stop)
				return
			}
		}
	}()

	return nil
}

// stopIfCurrent 仅在 stop 仍属于当前这次运行时才将任务标记为停止，
// 避免已退出的旧循环关掉重启后的新循环。
func (t *Task) stopIfCurrent(stop chan struct{}) {
	t.access.Lock()
	if t.running && t.stop == stop {
		t.running = false
		close(stop)
	}
	t.access.Unlock()
}

func (t *Task) Close() {
	t.access.Lock()
	if t.running {
		t.running = false
		close(t.stop)
	}
	t.access.Unlock()
}
