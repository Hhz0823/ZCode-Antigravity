package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	usageQueueBatchSize = 100
	maxUsageSamples     = 256
)

type gatewayUsageRecord struct {
	Timestamp      time.Time             `json:"timestamp"`
	LatencyMS      int64                 `json:"latency_ms"`
	TTFTMS         int64                 `json:"ttft_ms"`
	Provider       string                `json:"provider"`
	Model          string                `json:"model"`
	Alias          string                `json:"alias"`
	Failed         bool                  `json:"failed"`
	Generate       bool                  `json:"generate"`
	Tokens         gatewayUsageTokens    `json:"tokens"`
	TokenBreakdown gatewayTokenBreakdown `json:"token_breakdown"`
}

type gatewayUsageTokens struct {
	OutputTokens    int64 `json:"output_tokens"`
	ReasoningTokens int64 `json:"reasoning_tokens"`
	TotalTokens     int64 `json:"total_tokens"`
}

type gatewayTokenBreakdown struct {
	Output struct {
		TotalTokens        int64 `json:"total_tokens"`
		NonReasoningTokens int64 `json:"non_reasoning_tokens"`
		ReasoningTokens    int64 `json:"reasoning_tokens"`
	} `json:"output"`
}

type usageSample struct {
	Timestamp             time.Time `json:"timestamp"`
	Provider              string    `json:"provider"`
	Model                 string    `json:"model"`
	OutputTokens          int64     `json:"outputTokens"`
	NonReasoningTokens    int64     `json:"nonReasoningTokens"`
	ReasoningTokens       int64     `json:"reasoningTokens"`
	TotalTokens           int64     `json:"totalTokens"`
	LatencyMS             int64     `json:"latencyMs"`
	TTFTMS                int64     `json:"ttftMs"`
	GenerationMS          int64     `json:"generationMs"`
	OutputTokensPerSecond float64   `json:"outputTokensPerSecond"`
	SpeedBasis            string    `json:"speedBasis"`
}

type usageAggregate struct {
	Requests               int       `json:"requests"`
	OutputTokens           int64     `json:"outputTokens"`
	ReasoningTokens        int64     `json:"reasoningTokens"`
	AverageTokensPerSecond float64   `json:"averageTokensPerSecond"`
	TrackedFrom            time.Time `json:"trackedFrom,omitempty"`
}

type usageReport struct {
	Provider  string         `json:"provider"`
	Available bool           `json:"available"`
	Latest    *usageSample   `json:"latest,omitempty"`
	Total     usageAggregate `json:"total"`
	Recent    []usageSample  `json:"recent"`
	UpdatedAt time.Time      `json:"updatedAt"`
	Warning   string         `json:"warning,omitempty"`
}

type persistedUsage struct {
	Version int           `json:"version"`
	Samples []usageSample `json:"samples"`
}

type usageTracker struct {
	mu        sync.RWMutex
	path      string
	samples   []usageSample
	lastError string
}

func newUsageTracker(path string) *usageTracker {
	t := &usageTracker{path: path, samples: make([]usageSample, 0, maxUsageSamples)}
	raw, errRead := os.ReadFile(path)
	if errors.Is(errRead, os.ErrNotExist) {
		return t
	}
	if errRead != nil {
		t.lastError = "读取本地 Token 统计失败"
		return t
	}
	var persisted persistedUsage
	if errJSON := json.Unmarshal(raw, &persisted); errJSON != nil || persisted.Version != 1 {
		t.lastError = "本地 Token 统计格式无效，已从空记录继续"
		return t
	}
	if len(persisted.Samples) > maxUsageSamples {
		persisted.Samples = persisted.Samples[len(persisted.Samples)-maxUsageSamples:]
	}
	t.samples = append(t.samples, persisted.Samples...)
	return t
}

func usageSampleFromGateway(record gatewayUsageRecord) (usageSample, bool) {
	if record.Failed || !record.Generate {
		return usageSample{}, false
	}
	outputTokens := record.TokenBreakdown.Output.TotalTokens
	nonReasoning := record.TokenBreakdown.Output.NonReasoningTokens
	reasoning := record.TokenBreakdown.Output.ReasoningTokens
	if outputTokens <= 0 {
		outputTokens = record.Tokens.OutputTokens
		if record.Tokens.ReasoningTokens > 0 && record.Tokens.ReasoningTokens > outputTokens {
			outputTokens = record.Tokens.ReasoningTokens
		}
	}
	if reasoning <= 0 {
		reasoning = record.Tokens.ReasoningTokens
	}
	if nonReasoning <= 0 && outputTokens > reasoning {
		nonReasoning = outputTokens - reasoning
	}
	if outputTokens <= 0 || record.LatencyMS <= 0 {
		return usageSample{}, false
	}
	durationMS := record.LatencyMS
	basis := "effective"
	if record.TTFTMS > 0 && record.TTFTMS < record.LatencyMS {
		durationMS = record.LatencyMS - record.TTFTMS
		basis = "generation"
	}
	if durationMS <= 0 {
		return usageSample{}, false
	}
	model := strings.TrimSpace(record.Alias)
	if model == "" {
		model = strings.TrimSpace(record.Model)
	}
	when := record.Timestamp.UTC()
	if when.IsZero() {
		when = time.Now().UTC()
	}
	return usageSample{
		Timestamp:             when,
		Provider:              normalizeProvider(record.Provider),
		Model:                 model,
		OutputTokens:          outputTokens,
		NonReasoningTokens:    nonReasoning,
		ReasoningTokens:       reasoning,
		TotalTokens:           record.Tokens.TotalTokens,
		LatencyMS:             record.LatencyMS,
		TTFTMS:                record.TTFTMS,
		GenerationMS:          durationMS,
		OutputTokensPerSecond: float64(outputTokens) / (float64(durationMS) / 1000),
		SpeedBasis:            basis,
	}, true
}

func (t *usageTracker) consume(records []gatewayUsageRecord) {
	if t == nil || len(records) == 0 {
		return
	}
	samples := make([]usageSample, 0, len(records))
	for _, record := range records {
		if sample, ok := usageSampleFromGateway(record); ok {
			samples = append(samples, sample)
		}
	}
	if len(samples) == 0 {
		return
	}
	sort.SliceStable(samples, func(i, j int) bool { return samples[i].Timestamp.Before(samples[j].Timestamp) })
	t.mu.Lock()
	t.samples = append(t.samples, samples...)
	if len(t.samples) > maxUsageSamples {
		t.samples = append([]usageSample(nil), t.samples[len(t.samples)-maxUsageSamples:]...)
	}
	persisted := persistedUsage{Version: 1, Samples: append([]usageSample(nil), t.samples...)}
	t.lastError = ""
	t.mu.Unlock()
	if raw, errJSON := json.MarshalIndent(persisted, "", "  "); errJSON == nil {
		if errWrite := writeAtomic(t.path, append(raw, '\n'), 0o600); errWrite != nil {
			t.setError("保存本地 Token 统计失败")
		}
	}
}

func (t *usageTracker) setError(value string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.lastError = strings.TrimSpace(value)
	t.mu.Unlock()
}

func (t *usageTracker) report(provider string) usageReport {
	provider = normalizeProvider(provider)
	now := time.Now().UTC()
	report := usageReport{Provider: provider, Recent: []usageSample{}, UpdatedAt: now}
	if t == nil {
		return report
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	report.Warning = t.lastError
	var weightedSeconds float64
	for _, sample := range t.samples {
		if sample.Provider != provider {
			continue
		}
		report.Available = true
		report.Total.Requests++
		report.Total.OutputTokens += sample.OutputTokens
		report.Total.ReasoningTokens += sample.ReasoningTokens
		weightedSeconds += float64(sample.GenerationMS) / 1000
		if report.Total.TrackedFrom.IsZero() || sample.Timestamp.Before(report.Total.TrackedFrom) {
			report.Total.TrackedFrom = sample.Timestamp
		}
		copySample := sample
		report.Latest = &copySample
		report.Recent = append(report.Recent, sample)
	}
	if weightedSeconds > 0 {
		report.Total.AverageTokensPerSecond = float64(report.Total.OutputTokens) / weightedSeconds
	}
	if len(report.Recent) > 8 {
		report.Recent = append([]usageSample(nil), report.Recent[len(report.Recent)-8:]...)
	}
	return report
}

func (a *app) popUsageQueue(port int) ([]gatewayUsageRecord, error) {
	req, errRequest := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/v0/management/usage-queue?count=%d", a.gatewayURL(port), usageQueueBatchSize), nil)
	if errRequest != nil {
		return nil, errRequest
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("X-Management-Key", a.apiKey)
	client := httpClient(2500 * time.Millisecond)
	resp, errDo := client.Do(req)
	if errDo != nil {
		return nil, errDo
	}
	defer resp.Body.Close()
	raw, errRead := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if errRead != nil {
		return nil, errRead
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Token 统计接口返回 HTTP %d", resp.StatusCode)
	}
	var records []gatewayUsageRecord
	if errJSON := json.Unmarshal(raw, &records); errJSON != nil {
		return nil, fmt.Errorf("Token 统计响应无效: %w", errJSON)
	}
	return records, nil
}

func (g *guiRuntime) monitorUsage(done <-chan struct{}) {
	if g == nil || g.app == nil || g.usage == nil {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	configuredPort := 0
	poll := func() {
		current, errState := g.app.loadState()
		if errState != nil || current.Port <= 0 {
			return
		}
		if current.Port != configuredPort {
			var enabled map[string]bool
			if errGet := g.app.managementJSON(current.Port, http.MethodGet, "/v0/management/usage-statistics-enabled", nil, &enabled); errGet == nil {
				if !enabled["usage-statistics-enabled"] {
					if errPut := g.app.managementJSON(current.Port, http.MethodPut, "/v0/management/usage-statistics-enabled", map[string]bool{"value": true}, nil); errPut != nil {
						g.usage.setError("无法启用本地 Token 统计")
						return
					}
				}
				configuredPort = current.Port
			}
		}
		for {
			records, errPop := g.app.popUsageQueue(current.Port)
			if errPop != nil {
				return
			}
			g.usage.consume(records)
			if len(records) < usageQueueBatchSize {
				return
			}
		}
	}
	poll()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			poll()
		}
	}
}

func (g *guiRuntime) serveUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	provider := strings.TrimSpace(r.URL.Query().Get("provider"))
	if provider == "" {
		provider = g.currentProvider()
	}
	writeJSON(w, http.StatusOK, g.usage.report(provider))
}
