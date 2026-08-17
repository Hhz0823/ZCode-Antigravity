package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strings"
)

type traySnapshot struct {
	Provider  string
	Summary   string
	Detail    string
	Remaining *float64
}

type trayHooks struct {
	Open           func()
	Refresh        func() traySnapshot
	SelectProvider func(string)
	Updates        <-chan struct{}
	Quit           func()
}

func traySnapshotFromReport(provider string, report quotaReport) traySnapshot {
	provider = normalizeProvider(provider)
	name := "Antigravity"
	if provider == "xai" {
		name = "Grok"
	}
	snapshot := traySnapshot{Provider: provider, Summary: name + " 额度暂不可用", Detail: "点击刷新或打开额度面板"}
	if len(report.Accounts) == 0 {
		return snapshot
	}
	account := report.Accounts[0]
	labels := make([]string, 0, 3)
	remainingByLabel := make(map[string]float64)
	var lowest *float64
	for _, quotaAccount := range report.Accounts {
		for _, group := range quotaAccount.Groups {
			for _, bucket := range group.Buckets {
				if bucket.RemainingPercent == nil {
					continue
				}
				value := math.Max(0, math.Min(100, *bucket.RemainingPercent))
				if lowest == nil || value < *lowest {
					copyValue := value
					lowest = &copyValue
				}
				label := "额度"
				search := strings.ToLower(bucket.Name + " " + bucket.Window)
				switch {
				case strings.Contains(search, "week") || strings.Contains(search, "周"):
					label = "周"
				case strings.Contains(search, "5") || strings.Contains(search, "五"):
					label = "5小时"
				case strings.Contains(search, "month") || strings.Contains(search, "月"):
					label = "月"
				}
				previous, exists := remainingByLabel[label]
				if !exists {
					labels = append(labels, label)
				}
				if !exists || value < previous {
					remainingByLabel[label] = value
				}
			}
		}
	}
	parts := make([]string, 0, len(labels))
	for _, label := range labels {
		parts = append(parts, label+" "+formatTrayPercent(remainingByLabel[label]))
	}
	if len(parts) > 0 {
		snapshot.Summary = name + " · " + strings.Join(parts, " · ")
		snapshot.Detail = firstText(account.Plan, account.StatusMessage, account.Status)
		if len(report.Accounts) > 1 {
			snapshot.Detail = fmt.Sprintf("%d 个账号 · %s", len(report.Accounts), snapshot.Detail)
		}
		snapshot.Remaining = lowest
	} else if account.Error != "" {
		snapshot.Detail = account.Error
	}
	return snapshot
}

func formatTrayPercent(value float64) string {
	return strings.TrimSuffix(strings.TrimSuffix(strconvFormatFloat(value), ".0"), ".") + "%"
}

func strconvFormatFloat(value float64) string {
	if math.Abs(value-math.Round(value)) < 0.05 {
		return fmt.Sprintf("%.0f", value)
	}
	return fmt.Sprintf("%.1f", value)
}

func quotaTrayIcon(remaining *float64, provider string, template bool) []byte {
	const size = 44
	canvas := image.NewNRGBA(image.Rect(0, 0, size, size))
	center := float64(size-1) / 2
	remainingValue := 0.0
	if remaining != nil {
		remainingValue = math.Max(0, math.Min(100, *remaining))
	}
	track := color.NRGBA{R: 126, G: 143, B: 166, A: 115}
	accent := color.NRGBA{R: 17, G: 105, B: 236, A: 255}
	if normalizeProvider(provider) == "xai" {
		accent = color.NRGBA{R: 110, G: 55, B: 230, A: 255}
	}
	if remainingValue <= 15 && remaining != nil {
		accent = color.NRGBA{R: 220, G: 63, B: 55, A: 255}
	}
	if template {
		track = color.NRGBA{A: 75}
		accent = color.NRGBA{A: 255}
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)-center, float64(y)-center
			radius := math.Hypot(dx, dy)
			if radius < 15.2 || radius > 19.4 {
				continue
			}
			angle := math.Atan2(dx, -dy)
			if angle < 0 {
				angle += 2 * math.Pi
			}
			pixel := track
			if remaining != nil && angle <= 2*math.Pi*(remainingValue/100) {
				pixel = accent
			}
			canvas.SetNRGBA(x, y, pixel)
		}
	}
	// A small center mark stays crisp at 100–300% DPI.
	centerColor := accent
	if remaining == nil {
		centerColor = track
	}
	draw.Draw(canvas, image.Rect(19, 14, 25, 29), &image.Uniform{C: centerColor}, image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(15, 19, 29, 24), &image.Uniform{C: centerColor}, image.Point{}, draw.Src)
	var output bytes.Buffer
	_ = png.Encode(&output, canvas)
	return output.Bytes()
}
