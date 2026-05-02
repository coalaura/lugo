package lsp

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/coalaura/lugo/ast"
)

const (
	fiveMColdAllocBudgetRatio             = 1.25
	fiveMWarmReindexAllocBudgetRatio      = 0.15
	fiveMNativeActivationAllocBudgetRatio = 1.10
)

var fiveMPerfFixtureNames = []string{
	"plain_lua",
	"manifest_restricted",
	"resource_client_server_shared",
	"resource_dual_listed",
	"resource_exports",
	"resource_provides",
	"resource_bridges",
	"resource_natives",
	"mixed_workspace",
}

type fiveMPerfSample struct {
	Wall       time.Duration
	AllocBytes uint64
}

type fiveMPerfMetric struct {
	Samples          []fiveMPerfSample
	MedianWall       time.Duration
	MedianAllocBytes uint64
}

func TestFiveMPerfBudgets(t *testing.T) {
	coldFiveM := measureFiveMPerfMetric(t, 5, func(tb testing.TB) *fiveMFixtureHarness {
		return newFiveMPerfCorpusHarness(tb)
	}, func(tb testing.TB, h *fiveMFixtureHarness) {
		h.reindex()
	})

	coldPlainControl := measureFiveMPerfMetric(t, 5, func(tb testing.TB) *fiveMFixtureHarness {
		return newFiveMPlainControlHarness(tb)
	}, func(tb testing.TB, h *fiveMFixtureHarness) {
		h.reindex()
	})

	warmReindex := measureFiveMPerfMetric(t, 7, func(tb testing.TB) *fiveMFixtureHarness {
		h := newFiveMPerfCorpusHarness(tb)
		h.reindex()
		return h
	}, func(tb testing.TB, h *fiveMFixtureHarness) {
		h.reindex()
	})

	runtimeLookup := measureFiveMPerfMetric(t, 9, func(tb testing.TB) *fiveMFixtureHarness {
		return newFiveMNativePerfHarness(tb)
	}, func(tb testing.TB, h *fiveMFixtureHarness) {
		syms := lookupFiveMRuntimeSymbols(tb, h)
		if len(syms) == 0 || syms[0].URI != "std:///fivem/shared.lua" {
			tb.Fatalf("runtime lookup resolved to %+v, want std:///fivem/shared.lua", syms)
		}
		if got := countLoadedFiveMNativeBundles(h.server); got != len(fiveMNativeBundleNames) {
			tb.Fatalf("runtime lookup kept %d native bundles indexed, want %d", got, len(fiveMNativeBundleNames))
		}
	})

	nativeWarmLookup := measureFiveMPerfMetric(t, 9, func(tb testing.TB) *fiveMFixtureHarness {
		h := newFiveMNativePerfHarness(tb)
		wantURI := requireFiveMNativeBundleURI(tb, h.server, "natives_universal.lua")
		ctx := h.resolve("native_client_call")
		if ctx == nil || ctx.TargetURI != wantURI {
			tb.Fatalf("native warm-up resolved to %+v, want %s", ctx, wantURI)
		}
		return h
	}, func(tb testing.TB, h *fiveMFixtureHarness) {
		wantURI := requireFiveMNativeBundleURI(tb, h.server, "natives_universal.lua")
		ctx := h.resolve("native_client_call")
		if ctx == nil || ctx.TargetURI != wantURI {
			tb.Fatalf("native warm lookup resolved to %+v, want %s", ctx, wantURI)
		}
		if got := countLoadedFiveMNativeBundles(h.server); got != len(fiveMNativeBundleNames) {
			tb.Fatalf("native warm lookup kept %d native bundles indexed, want %d", got, len(fiveMNativeBundleNames))
		}
	})

	nativeActivation := measureFiveMPerfMetric(t, 9, func(tb testing.TB) *fiveMFixtureHarness {
		return newFiveMNativePerfHarness(tb)
	}, func(tb testing.TB, h *fiveMFixtureHarness) {
		wantURI := requireFiveMNativeBundleURI(tb, h.server, "natives_universal.lua")
		ctx := h.resolve("native_client_call")
		if ctx == nil || ctx.TargetURI != wantURI {
			tb.Fatalf("native activation resolved to %+v, want %s", ctx, wantURI)
		}
		if got := countLoadedFiveMNativeBundles(h.server); got != len(fiveMNativeBundleNames) {
			tb.Fatalf("native activation kept %d native bundles indexed, want %d", got, len(fiveMNativeBundleNames))
		}
	})

	coldWallRatio := ratioDuration(coldFiveM.MedianWall, coldPlainControl.MedianWall)
	coldAllocRatio := ratioUint64(coldFiveM.MedianAllocBytes, coldPlainControl.MedianAllocBytes)
	warmAllocRatio := ratioUint64(warmReindex.MedianAllocBytes, coldFiveM.MedianAllocBytes)
	nativeActivationAllocRatio := ratioUint64(nativeActivation.MedianAllocBytes, nativeWarmLookup.MedianAllocBytes)

	t.Logf("FiveM cold index median: wall=%s alloc=%dB", coldFiveM.MedianWall, coldFiveM.MedianAllocBytes)
	t.Logf("plain-Lua cold control median: wall=%s alloc=%dB", coldPlainControl.MedianWall, coldPlainControl.MedianAllocBytes)
	t.Logf("FiveM warm reindex median: wall=%s alloc=%dB", warmReindex.MedianWall, warmReindex.MedianAllocBytes)
	t.Logf("FiveM runtime lookup median: wall=%s alloc=%dB", runtimeLookup.MedianWall, runtimeLookup.MedianAllocBytes)
	t.Logf("FiveM warm native lookup median: wall=%s alloc=%dB", nativeWarmLookup.MedianWall, nativeWarmLookup.MedianAllocBytes)
	t.Logf("FiveM native activation median: wall=%s alloc=%dB", nativeActivation.MedianWall, nativeActivation.MedianAllocBytes)
	t.Logf("FiveM cold index wall ratio: %.3fx", coldWallRatio)

	if coldAllocRatio > fiveMColdAllocBudgetRatio {
		t.Fatalf("FiveM cold index allocation ratio %.3fx exceeded budget %.2fx", coldAllocRatio, fiveMColdAllocBudgetRatio)
	}
	if warmAllocRatio > fiveMWarmReindexAllocBudgetRatio {
		t.Fatalf("FiveM warm reindex allocation ratio %.3fx exceeded budget %.2fx", warmAllocRatio, fiveMWarmReindexAllocBudgetRatio)
	}
	if nativeActivationAllocRatio > fiveMNativeActivationAllocBudgetRatio {
		t.Fatalf("FiveM native activation allocation ratio %.3fx exceeded budget %.2fx", nativeActivationAllocRatio, fiveMNativeActivationAllocBudgetRatio)
	}
}

func BenchmarkFiveM(b *testing.B) {
	b.Run("ColdIndexFiveM", func(b *testing.B) {
		benchmarkFiveMColdIndex(b, newFiveMPerfCorpusHarness)
	})

	b.Run("ColdIndexPlainLuaControl", func(b *testing.B) {
		benchmarkFiveMColdIndex(b, newFiveMPlainControlHarness)
	})

	b.Run("WarmReindex", func(b *testing.B) {
		h := newFiveMPerfCorpusHarness(b)
		h.reindex()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			h.reindex()
		}
	})

	b.Run("RuntimeLookup", func(b *testing.B) {
		h := newFiveMNativePerfHarness(b)

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			syms := lookupFiveMRuntimeSymbols(b, h)
			if len(syms) == 0 || syms[0].URI != "std:///fivem/shared.lua" {
				b.Fatalf("runtime lookup resolved to %+v, want std:///fivem/shared.lua", syms)
			}
		}
	})

	b.Run("NativeActivation", func(b *testing.B) {
		h := newFiveMNativePerfHarness(b)
		wantURI := requireFiveMNativeBundleURI(b, h.server, "natives_universal.lua")

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := h.resolve("native_client_call")
			if ctx == nil || ctx.TargetURI != wantURI {
				b.Fatalf("native activation resolved to %+v, want %s", ctx, wantURI)
			}
		}
	})
}

func benchmarkFiveMColdIndex(b *testing.B, setup func(testing.TB) *fiveMFixtureHarness) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		h := setup(b)
		b.StartTimer()
		h.reindex()
	}
}

func newFiveMPerfCorpusHarness(tb testing.TB) *fiveMFixtureHarness {
	tb.Helper()
	return newFiveMFixtureHarnessWithoutIndex(tb, fiveMPerfFixtureNames...)
}

func newFiveMPlainControlHarness(tb testing.TB) *fiveMFixtureHarness {
	tb.Helper()
	h := newFiveMFixtureHarnessWithoutIndex(tb, fiveMPerfFixtureNames...)
	renameFiveMManifestFiles(tb, h.root)
	return h
}

func newFiveMNativePerfHarness(tb testing.TB) *fiveMFixtureHarness {
	tb.Helper()

	h := newFiveMFixtureHarnessWithoutIndex(tb, "resource_natives")
	h.reindex()

	if got := countLoadedFiveMNativeBundles(h.server); got != len(fiveMNativeBundleNames) {
		tb.Fatalf("loaded native bundles after native perf setup = %d, want %d", got, len(fiveMNativeBundleNames))
	}

	return h
}

func lookupFiveMRuntimeSymbols(tb testing.TB, h *fiveMFixtureHarness) []GlobalSymbol {
	tb.Helper()

	doc := h.docForMarker("native_client_call")
	syms, ok := h.server.getGlobalSymbols(doc, 0, ast.HashBytes([]byte("Citizen")))
	if !ok {
		tb.Fatal("runtime lookup did not return the Citizen global")
	}

	return syms
}

func renameFiveMManifestFiles(tb testing.TB, root string) {
	tb.Helper()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		var target string
		switch d.Name() {
		case "fxmanifest.lua":
			target = filepath.Join(filepath.Dir(path), "manifest.lua")
		case "__resource.lua":
			target = filepath.Join(filepath.Dir(path), "resource.lua")
		default:
			return nil
		}

		return os.Rename(path, target)
	})
	if err != nil {
		tb.Fatalf("rename FiveM manifests under %s: %v", root, err)
	}
}

func measureFiveMPerfMetric(tb testing.TB, runs int, setup func(testing.TB) *fiveMFixtureHarness, op func(testing.TB, *fiveMFixtureHarness)) fiveMPerfMetric {
	tb.Helper()

	samples := make([]fiveMPerfSample, 0, runs)
	for i := 0; i < runs; i++ {
		h := setup(tb)

		runtime.GC()

		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		op(tb, h)
		wall := time.Since(start)
		runtime.ReadMemStats(&after)

		samples = append(samples, fiveMPerfSample{
			Wall:       wall,
			AllocBytes: after.TotalAlloc - before.TotalAlloc,
		})
	}

	return summarizeFiveMPerfSamples(samples)
}

func summarizeFiveMPerfSamples(samples []fiveMPerfSample) fiveMPerfMetric {
	wallSamples := make([]time.Duration, 0, len(samples))
	allocSamples := make([]uint64, 0, len(samples))
	for _, sample := range samples {
		wallSamples = append(wallSamples, sample.Wall)
		allocSamples = append(allocSamples, sample.AllocBytes)
	}

	return fiveMPerfMetric{
		Samples:          samples,
		MedianWall:       medianDuration(wallSamples),
		MedianAllocBytes: medianUint64(allocSamples),
	}
}

func medianDuration(values []time.Duration) time.Duration {
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[len(sorted)/2]
}

func medianUint64(values []uint64) uint64 {
	sorted := append([]uint64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[len(sorted)/2]
}

func ratioDuration(lhs, rhs time.Duration) float64 {
	if rhs <= 0 {
		return 0
	}
	return float64(lhs) / float64(rhs)
}

func ratioUint64(lhs, rhs uint64) float64 {
	if rhs == 0 {
		return 0
	}
	return float64(lhs) / float64(rhs)
}
