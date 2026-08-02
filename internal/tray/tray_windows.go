//go:build windows

package tray

import (
	"sync"

	"github.com/getlantern/systray"
)

type Actions struct {
	Show  func()
	Start func()
	Stop  func()
	Quit  func()
}

type Controller struct {
	once   sync.Once
	mu     sync.Mutex
	status *systray.MenuItem
	start  *systray.MenuItem
	stop   *systray.MenuItem
}

func (c *Controller) Start(icon []byte, actions Actions) {
	c.once.Do(func() {
		go systray.Run(func() {
			systray.SetIcon(icon)
			systray.SetTitle("WinRouter")
			systray.SetTooltip("WinRouter - 已停止")
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
						actions.Show()
					case <-start.ClickedCh:
						actions.Start()
					case <-stop.ClickedCh:
						actions.Stop()
					case <-quit.ClickedCh:
						actions.Quit()
						return
					}
				}
			}()
		}, func() {})
	})
}

func (c *Controller) Stop() { systray.Quit() }

func (c *Controller) SetStatus(label string, running bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.status == nil {
		return
	}
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
