package loadtest

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"testing"
	"time"

	"proxy-kp/pkg/balancer"
	"proxy-kp/pkg/balancer/leastconn"
	"proxy-kp/pkg/balancer/srr"
)

type algoFactory func() balancer.Balancer

type scenario struct {
	name     string
	backends int
	workers  int
}

type benchResult struct {
	scenario
	opsPerSec float64
	p50       time.Duration
	p95       time.Duration
	p99       time.Duration
	p999      time.Duration
	samples   int
}

type runConfig struct {
	factory    algoFactory
	backends   int
	workers    int
	iterations int
	burstEvery int
	setupFn    func(balancer.Balancer)
}

func run(cfg runConfig) benchResult {
	bal := cfg.factory()
	if cfg.setupFn != nil {
		cfg.setupFn(bal)
	} else {
		rng := rand.New(rand.NewSource(42))
		for i := 0; i < cfg.backends; i++ {
			bal.AddBackend(balancer.NewBackend(
				fmt.Sprintf("http://node%d:8001", i),
				rng.Intn(9)+1,
			))
		}
	}

	var mu sync.Mutex
	allDurations := make([]time.Duration, 0, cfg.iterations)
	startTotal := time.Now()

	if cfg.workers <= 1 {
		for i := 0; i < cfg.iterations; i++ {
			start := time.Now()
			be, err := bal.NextBackend()
			if err != nil {
				continue
			}
			bal.Acquire(be.URL)
			bal.Release(be.URL)
			allDurations = append(allDurations, time.Since(start))
		}
	} else {
		var wg sync.WaitGroup
		perWorker := cfg.iterations / cfg.workers
		rem := cfg.iterations % cfg.workers

		burstGate := make(chan struct{}, cfg.workers)
		if cfg.burstEvery > 0 {
			for i := 0; i < cfg.workers; i++ {
				burstGate <- struct{}{}
			}
		}

		for w := 0; w < cfg.workers; w++ {
			wg.Add(1)
			count := perWorker
			if w < rem {
				count++
			}
			go func(n int) {
				defer wg.Done()
				local := make([]time.Duration, 0, n)
				for i := 0; i < n; i++ {
					if cfg.burstEvery > 0 && i%cfg.burstEvery == 0 {
						burstGate <- struct{}{}
					}
					if cfg.burstEvery > 0 && i%cfg.burstEvery == 0 && len(burstGate) == cfg.workers {
						for len(burstGate) > 0 {
							<-burstGate
						}
					}

					start := time.Now()
					be, err := bal.NextBackend()
					if err != nil {
						continue
					}
					bal.Acquire(be.URL)
					bal.Release(be.URL)
					local = append(local, time.Since(start))
				}
				mu.Lock()
				allDurations = append(allDurations, local...)
				mu.Unlock()
			}(count)
		}
		wg.Wait()
	}

	elapsed := time.Since(startTotal)
	sort.Slice(allDurations, func(i, j int) bool {
		return allDurations[i] < allDurations[j]
	})

	n := len(allDurations)
	res := benchResult{
		scenario:  scenario{name: "", backends: cfg.backends, workers: cfg.workers},
		samples:   n,
		opsPerSec: float64(n) / elapsed.Seconds(),
	}
	if n > 0 {
		res.p50 = allDurations[n*50/100]
		res.p95 = allDurations[n*95/100]
		res.p99 = allDurations[n*99/100]
		res.p999 = allDurations[n*999/1000]
	}
	return res
}

func fmtDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%.0fns", float64(d.Nanoseconds()))
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.1fµs", float64(d.Nanoseconds())/1000)
	}
	return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1e6)
}

func fmtOps(n float64) string {
	switch {
	case n >= 1e6:
		return fmt.Sprintf("%.1fM", n/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.1fK", n/1e3)
	default:
		return fmt.Sprintf("%.0f", n)
	}
}

func printHeader(name string) {
	fmt.Println()
	fmt.Printf("─── %s ─%s", name, repeatChar('─', 92-len(name)))
	fmt.Println()
}

func printTableHeader() {
	fmt.Printf("%-28s %-5s %-4s %12s %10s %10s %10s %10s\n",
		"Scenario", "Bcknd", "Work", "RPS", "P50", "P95", "P99", "P999")
	fmt.Println(repeatChar('─', 96))
}

func printRow(name string, r benchResult) {
	fmt.Printf("%-28s %-5d %-4d %12s %10s %10s %10s %10s\n",
		name, r.backends, r.workers,
		fmtOps(r.opsPerSec)+"/s",
		fmtDuration(r.p50), fmtDuration(r.p95),
		fmtDuration(r.p99), fmtDuration(r.p999))
}

func printDivider() {
	fmt.Println(repeatChar('─', 96))
}

func repeatChar(ch rune, n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = ch
	}
	return string(b)
}

func TestLoadCompare(t *testing.T) {
	scenarios := []struct {
		name     string
		factory  algoFactory
		backends int
		workers  int
		iters    int
	}{
		{"SRR  3 backends", func() balancer.Balancer { return srr.New() }, 3, 1, 200000},
		{"LC   3 backends", func() balancer.Balancer { return leastconn.New() }, 3, 1, 200000},
		{"SRR  3 backends (×8)", func() balancer.Balancer { return srr.New() }, 3, 8, 200000},
		{"LC   3 backends (×8)", func() balancer.Balancer { return leastconn.New() }, 3, 8, 200000},
		{"SRR  10 backends", func() balancer.Balancer { return srr.New() }, 10, 1, 200000},
		{"LC   10 backends", func() balancer.Balancer { return leastconn.New() }, 10, 1, 200000},
		{"SRR  10 backends (×16)", func() balancer.Balancer { return srr.New() }, 10, 16, 200000},
		{"LC   10 backends (×16)", func() balancer.Balancer { return leastconn.New() }, 10, 16, 200000},
		{"SRR  20 backends (×16)", func() balancer.Balancer { return srr.New() }, 20, 16, 200000},
		{"LC   20 backends (×16)", func() balancer.Balancer { return leastconn.New() }, 20, 16, 200000},
	}

	printHeader("Balancer Load Test")
	printTableHeader()

	for _, sc := range scenarios {
		res := run(runConfig{
			factory:    sc.factory,
			backends:   sc.backends,
			workers:    sc.workers,
			iterations: sc.iters,
		})
		printRow(sc.name, res)
	}
	printDivider()

	fmt.Println()
	pairs := []struct{ a, b int; label string }{
		{0, 1, "seq 3 backends"},
		{2, 3, "par 3 backends (×8)"},
		{4, 5, "seq 10 backends"},
		{6, 7, "par 10 backends (×16)"},
		{8, 9, "par 20 backends (×16)"},
	}
	fmt.Println("Comparisons:")
	for _, p := range pairs {
		s := scenarios[p.a]
		a := run(runConfig{factory: s.factory, backends: s.backends, workers: s.workers, iterations: s.iters})
		b := run(runConfig{factory: scenarios[p.b].factory, backends: scenarios[p.b].backends, workers: scenarios[p.b].workers, iterations: scenarios[p.b].iters})
		fmt.Printf("  %s: SRR %s/s vs LC %s/s  (ratio %.2fx)\n",
			p.label, fmtOps(a.opsPerSec), fmtOps(b.opsPerSec), a.opsPerSec/b.opsPerSec)
	}
	fmt.Println()
	fmt.Println("Notes:")
	fmt.Println("  LC = LeastConnections (weighted). SRR = Smooth Round Robin.")
	fmt.Println("  All backends have random weight [1..10].")
	fmt.Println("  Scenarios marked ×N run with N parallel goroutines.")
	fmt.Println(repeatChar('─', 96))
	fmt.Println()
}

func TestLoad_UnhealthyScenario(t *testing.T) {
	printHeader("Unhealthy Backends (4/6 healthy)")
	printTableHeader()

	for _, f := range []struct {
		name    string
		factory algoFactory
	}{
		{"SRR  partial unhealthy ", func() balancer.Balancer { return srr.New() }},
		{"LC   partial unhealthy ", func() balancer.Balancer { return leastconn.New() }},
		{"SRR  partial unhealthy ×8", func() balancer.Balancer { return srr.New() }},
		{"LC   partial unhealthy ×8", func() balancer.Balancer { return leastconn.New() }},
	} {
		w := 1
		if len(f.name) > 22 {
			w = 8
		}
		res := run(runConfig{
			factory:    f.factory,
			backends:   6,
			workers:    w,
			iterations: 100000,
			setupFn: func(bal balancer.Balancer) {
				for i := 0; i < 6; i++ {
					be := balancer.NewBackend(fmt.Sprintf("http://n%d:8001", i), 10)
					if i >= 4 {
						be.SetHealthy(false)
					}
					bal.AddBackend(be)
				}
			},
		})
		printRow(f.name, res)
	}
	printDivider()
	fmt.Println()
}

func TestLoad_ContentionScaling(t *testing.T) {
	workerLevels := []int{1, 2, 4, 8, 16, 32}

	printHeader("Contention Scaling (8 backends)")
	fmt.Printf("%-6s %10s %12s %10s %10s %10s\n",
		"Algo", "Workers", "RPS", "P50", "P95", "P99")
	fmt.Println(repeatChar('─', 63))

	for _, a := range []struct {
		name    string
		factory algoFactory
	}{
		{"SRR", func() balancer.Balancer { return srr.New() }},
		{"LC", func() balancer.Balancer { return leastconn.New() }},
	} {
		for _, w := range workerLevels {
			res := run(runConfig{
				factory:    a.factory,
				backends:   8,
				workers:    w,
				iterations: 200000,
			})
			fmt.Printf("%-6s %10d %12s %10s %10s %10s\n",
				a.name, w,
				fmtOps(res.opsPerSec)+"/s",
				fmtDuration(res.p50), fmtDuration(res.p95), fmtDuration(res.p99))
		}
		if a.name == "SRR" {
			fmt.Println(repeatChar('─', 63))
		}
	}
	fmt.Println(repeatChar('─', 63))
	fmt.Println()
}

func TestLoad_BurstPattern(t *testing.T) {
	printHeader("Burst Pattern (5 backends, ×8 workers, sync gap every 100 ops)")
	fmt.Printf("%-12s %12s %10s %10s %10s %10s %12s\n",
		"Algo", "RPS", "P50", "P95", "P99", "P999", "Max(jitter)")
	fmt.Println(repeatChar('─', 80))

	for _, a := range []struct {
		name    string
		factory algoFactory
	}{
		{"SRR burst", func() balancer.Balancer { return srr.New() }},
		{"LC  burst", func() balancer.Balancer { return leastconn.New() }},
	} {
		res := run(runConfig{
			factory:    a.factory,
			backends:   5,
			workers:    8,
			iterations: 100000,
			burstEvery: 100,
		})
		fmt.Printf("%-12s %12s %10s %10s %10s %10s %12s\n",
			a.name,
			fmtOps(res.opsPerSec)+"/s",
			fmtDuration(res.p50), fmtDuration(res.p95),
			fmtDuration(res.p99), fmtDuration(res.p999),
			fmtDuration(res.p999))
	}
	fmt.Println(repeatChar('─', 80))
	fmt.Println()
}

func TestLoad_LatencyStability(t *testing.T) {
	algos := []struct {
		name    string
		factory algoFactory
	}{
		{"SRR", func() balancer.Balancer { return srr.New() }},
		{"LC", func() balancer.Balancer { return leastconn.New() }},
	}

	printHeader("Latency Stability — stddev & variance (6 backends, ×4 workers)")
	fmt.Println()

	for _, a := range algos {
		res := run(runConfig{
			factory:    a.factory,
			backends:   6,
			workers:    4,
			iterations: 100000,
		})

		if res.samples == 0 {
			continue
		}

		mean := float64(res.p50.Nanoseconds())

		bal := a.factory()
		for i := 0; i < 6; i++ {
			bal.AddBackend(balancer.NewBackend(
				fmt.Sprintf("http://n%d:8001", i),
				rand.Intn(9)+1,
			))
		}

		var mu sync.Mutex
		var allDurations []time.Duration
		var wg sync.WaitGroup
		perWorker := 100000 / 4
		for wi := 0; wi < 4; wi++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				local := make([]time.Duration, 0, perWorker)
				for i := 0; i < perWorker; i++ {
					start := time.Now()
					be, err := bal.NextBackend()
					if err != nil {
						continue
					}
					bal.Acquire(be.URL)
					bal.Release(be.URL)
					local = append(local, time.Since(start))
				}
				mu.Lock()
				allDurations = append(allDurations, local...)
				mu.Unlock()
			}()
		}
		wg.Wait()

		n := len(allDurations)
		var sum float64
		for _, d := range allDurations {
			sum += float64(d.Nanoseconds())
		}
		mean = sum / float64(n)

		sort.Slice(allDurations, func(i, j int) bool {
			return allDurations[i] < allDurations[j]
		})

		var varianceSum float64
		for _, d := range allDurations {
			diff := float64(d.Nanoseconds()) - mean
			varianceSum += diff * diff
		}
		variance := varianceSum / float64(n)
		stddev := math.Sqrt(variance)

		p50 := allDurations[n*50/100]
		p95 := allDurations[n*95/100]
		p99 := allDurations[n*99/100]

		fmt.Printf("  %s:\n", a.name)
		fmt.Printf("    Mean:    %.1fns\n", mean)
		fmt.Printf("    StdDev:  %.1fns\n", stddev)
		fmt.Printf("    Variance: %.0f\n", variance)
		fmt.Printf("    P50:     %s\n", fmtDuration(p50))
		fmt.Printf("    P95:     %s\n", fmtDuration(p95))
		fmt.Printf("    P99:     %s\n", fmtDuration(p99))
		fmt.Printf("    RPS:     %s\n", fmtOps(res.opsPerSec)+"/s")
		fmt.Println()
	}
}
