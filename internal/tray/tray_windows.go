//go:build windows

package tray

import (
	"runtime"
	"sync"
	"syscall"
	"time"

	"fyne.io/systray"
)

var getDoubleClickTime = syscall.NewLazyDLL("user32.dll").NewProc("GetDoubleClickTime")

type Actions struct {
	Show   func()
	Toggle func()
	Start  func()
	Stop   func()
	Quit   func()
}

type Controller struct {
	once      sync.Once
	mu        sync.Mutex
	status    *systray.MenuItem
	start     *systray.MenuItem
	stop      *systray.MenuItem
	label     string
	running   bool
	statusSet bool
	lastTap   time.Time
}

func (c *Controller) Start(icon []byte, actions Actions) {
	c.once.Do(func() {
		go func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			systray.Run(func() {
				systray.SetIcon(icon)
				systray.SetTitle("WinRouter")
				systray.SetTooltip("WinRouter - 已停止")
				systray.SetOnTapped(func() {
					if c.registerLeftTap(time.Now(), doubleClickInterval()) {
						go actions.Toggle()
					}
				})
				status := systray.AddMenuItem("状态：已停止", "当前分流核心状态")
				status.Disable()
				systray.AddSeparator()
				show := systray.AddMenuItem("打开主界面", "显示 WinRouter 主窗口")
				start := systray.AddMenuItem("启动核心", "验证配置并请求管理员授权")
				stop := systray.AddMenuItem("停止核心", "停止分流并恢复系统网络")
				stop.Disable()
				c.mu.Lock()
				c.status, c.start, c.stop = status, start, stop
				c.mu.Unlock()
				systray.AddSeparator()
				quit := systray.AddMenuItem("退出 WinRouter", "停止核心并退出应用")

				go func() {
					for {
						select {
						case <-show.ClickedCh:
							go actions.Show()
						case <-start.ClickedCh:
							go actions.Start()
						case <-stop.ClickedCh:
							go actions.Stop()
						case <-quit.ClickedCh:
							actions.Quit()
							return
						}
					}
				}()
			}, func() {})
		}()
	})
}

func (c *Controller) registerLeftTap(now time.Time, interval time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.lastTap.IsZero() {
		elapsed := now.Sub(c.lastTap)
		if elapsed >= 0 && elapsed <= interval {
			c.lastTap = time.Time{}
			return true
		}
	}
	c.lastTap = now
	return false
}

func doubleClickInterval() time.Duration {
	milliseconds, _, _ := getDoubleClickTime.Call()
	if milliseconds == 0 {
		milliseconds = 500
	}
	return time.Duration(milliseconds) * time.Millisecond
}

func (c *Controller) Stop() { systray.Quit() }

func (c *Controller) SetStatus(label string, running bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.status == nil {
		return
	}
	if c.statusSet && c.label == label && c.running == running {
		return
	}
	c.label, c.running, c.statusSet = label, running, true
	c.status.SetTitle("状态：" + label)
	systray.SetTooltip("WinRouter - " + label)
	if running {
		c.start.Disable()
		c.stop.Enable()
	} else {
		c.start.Enable()
		c.stop.Disable()
	}
}
