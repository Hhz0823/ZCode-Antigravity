//go:build windows || darwin

package main

import (
	"runtime"
	"sync"
	"time"

	"github.com/gogpu/systray"
)

func platformTraySupported() bool { return true }

func runPlatformTray(done <-chan struct{}, hooks trayHooks) error {
	tray := systray.New()
	menu := systray.NewMenu()
	refreshCh := make(chan struct{}, 1)
	quotaItem := menu.Add("额度：正在读取…", func() {
		if hooks.Open != nil {
			hooks.Open()
		}
	})
	detailItem := menu.Add("本地安全网关", nil)
	detailItem.SetDisabled(true)
	menu.AddSeparator()
	var antigravityItem, grokItem *systray.MenuItem
	requestProvider := func(provider string) {
		if hooks.SelectProvider != nil {
			hooks.SelectProvider(provider)
		}
		if antigravityItem != nil && grokItem != nil {
			antigravityItem.SetChecked(provider == "antigravity")
			grokItem.SetChecked(provider == "xai")
		}
		select {
		case refreshCh <- struct{}{}:
		default:
		}
	}
	antigravityItem = menu.AddCheckbox("使用 Antigravity", true, func() {
		requestProvider("antigravity")
	})
	grokItem = menu.AddCheckbox("使用 Grok", false, func() {
		requestProvider("xai")
	})
	menu.AddSeparator()
	menu.Add("打开额度面板", func() {
		if hooks.Open != nil {
			hooks.Open()
		}
	})
	menu.Add("刷新额度", func() {
		select {
		case refreshCh <- struct{}{}:
		default:
		}
	})
	menu.AddSeparator()
	var removeOnce sync.Once
	remove := func() { removeOnce.Do(tray.Remove) }
	menu.Add("退出小组件", func() {
		if hooks.Quit != nil {
			hooks.Quit()
		}
		remove()
	})

	template := runtime.GOOS == "darwin"
	initialIcon := quotaTrayIcon(nil, "antigravity", template)
	tray.SetIcon(initialIcon).SetTooltip("ZCode 额度：正在读取").SetMenu(menu)
	if template {
		tray.SetTemplateIcon(initialIcon)
	}
	tray.OnClick(func() {
		if hooks.Open != nil {
			hooks.Open()
		}
	})
	tray.Show()

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		refresh := func() {
			if hooks.Refresh == nil {
				return
			}
			snapshot := hooks.Refresh()
			antigravityItem.SetChecked(snapshot.Provider != "xai")
			grokItem.SetChecked(snapshot.Provider == "xai")
			quotaItem.SetLabel(snapshot.Summary)
			detailItem.SetLabel(snapshot.Detail)
			tray.SetTooltip(snapshot.Summary)
			icon := quotaTrayIcon(snapshot.Remaining, snapshot.Provider, template)
			tray.SetIcon(icon)
			if template {
				tray.SetTemplateIcon(icon)
			}
		}
		refresh()
		for {
			select {
			case <-done:
				remove()
				return
			case <-ticker.C:
				refresh()
			case <-refreshCh:
				refresh()
			case <-hooks.Updates:
				refresh()
			}
		}
	}()
	return tray.Run()
}
