package main

import (
	"math"
	"path/filepath"
	"testing"
	"time"
)

func TestUsageSampleUsesCanonicalOutputAndGenerationDuration(t *testing.T) {
	record := gatewayUsageRecord{
		Timestamp: time.Date(2026, 8, 17, 1, 2, 3, 0, time.UTC),
		LatencyMS: 5000,
		TTFTMS:    1000,
		Provider:  "antigravity",
		Model:     "gemini-3.7-flash",
		Alias:     "gemini-3.7-flash",
		Generate:  true,
		Tokens: gatewayUsageTokens{
			OutputTokens:    120,
			ReasoningTokens: 30,
			TotalTokens:     600,
		},
	}
	record.TokenBreakdown.Output.TotalTokens = 150
	record.TokenBreakdown.Output.NonReasoningTokens = 120
	record.TokenBreakdown.Output.ReasoningTokens = 30

	sample, ok := usageSampleFromGateway(record)
	if !ok {
		t.Fatal("expected a usage sample")
	}
	if sample.OutputTokens != 150 || sample.NonReasoningTokens != 120 || sample.ReasoningTokens != 30 {
		t.Fatalf("unexpected output breakdown: %+v", sample)
	}
	if sample.GenerationMS != 4000 || sample.SpeedBasis != "generation" {
		t.Fatalf("unexpected generation timing: %+v", sample)
	}
	if math.Abs(sample.OutputTokensPerSecond-37.5) > 0.001 {
		t.Fatalf("tokens/s = %f, want 37.5", sample.OutputTokensPerSecond)
	}
}

func TestUsageTrackerSeparatesProvidersAndPersistsSanitizedSamples(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage-metrics.json")
	tracker := newUsageTracker(path)
	now := time.Now().UTC()
	records := []gatewayUsageRecord{
		{Timestamp: now.Add(-time.Second), LatencyMS: 2000, Provider: "antigravity", Model: "gemini", Generate: true, Tokens: gatewayUsageTokens{OutputTokens: 20}},
		{Timestamp: now, LatencyMS: 1000, TTFTMS: 500, Provider: "xai", Model: "grok", Generate: true, Tokens: gatewayUsageTokens{OutputTokens: 25}},
		{Timestamp: now, LatencyMS: 1000, Provider: "xai", Model: "failed", Generate: true, Failed: true, Tokens: gatewayUsageTokens{OutputTokens: 999}},
	}
	tracker.consume(records)

	antigravity := tracker.report("antigravity")
	if !antigravity.Available || antigravity.Total.Requests != 1 || antigravity.Total.OutputTokens != 20 {
		t.Fatalf("unexpected Antigravity report: %+v", antigravity)
	}
	grok := tracker.report("grok")
	if !grok.Available || grok.Total.Requests != 1 || grok.Latest == nil || grok.Latest.OutputTokens != 25 {
		t.Fatalf("unexpected Grok report: %+v", grok)
	}

	reloaded := newUsageTracker(path).report("xai")
	if reloaded.Total.Requests != 1 || reloaded.Latest == nil || reloaded.Latest.Model != "grok" {
		t.Fatalf("persisted report was not restored: %+v", reloaded)
	}
}
