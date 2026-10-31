package node

import (
	"testing"
	"time"

	"github.com/InazumaV/V2bX/api/panel"
	"github.com/InazumaV/V2bX/common/task"
)

func newIdleTask(interval time.Duration) *task.Task {
	return &task.Task{Interval: interval, Execute: func() error { return nil }}
}

func TestUpdateTaskIntervals(t *testing.T) {
	cases := []struct {
		name               string
		pull, push         time.Duration
		wantPull, wantPush time.Duration
	}{
		// 回归用例：推送间隔曾被错误地设成拉取间隔
		{"pull 与 push 不同", 10 * time.Second, 30 * time.Second, 10 * time.Second, 30 * time.Second},
		{"只改 push", 60 * time.Second, 15 * time.Second, 60 * time.Second, 15 * time.Second},
		{"0 表示保持不变", 0, 0, 60 * time.Second, 60 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Controller{
				nodeInfoMonitorPeriodic: newIdleTask(60 * time.Second),
				userReportPeriodic:      newIdleTask(60 * time.Second),
			}
			_ = c.nodeInfoMonitorPeriodic.Start(false)
			_ = c.userReportPeriodic.Start(false)
			defer c.nodeInfoMonitorPeriodic.Close()
			defer c.userReportPeriodic.Close()

			c.updateTaskIntervals(&panel.NodeInfo{PullInterval: tc.pull, PushInterval: tc.push})

			if got := c.nodeInfoMonitorPeriodic.Interval; got != tc.wantPull {
				t.Errorf("pull interval = %v, want %v", got, tc.wantPull)
			}
			if got := c.userReportPeriodic.Interval; got != tc.wantPush {
				t.Errorf("push interval = %v, want %v", got, tc.wantPush)
			}
		})
	}
}
