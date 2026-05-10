package main

import "testing"

func TestBuildSeckillWorkloadSingleHotActivity(t *testing.T) {
	workload, err := buildSeckillWorkload(100, 100, 20, 100)
	if err != nil {
		t.Fatalf("build workload failed: %v", err)
	}

	if got := len(workload.Requests); got != 100 {
		t.Fatalf("request count=%d, want 100", got)
	}
	if workload.ColdRequests != 0 {
		t.Fatalf("cold requests=%d, want 0", workload.ColdRequests)
	}
	if workload.DuplicateRequests != 20 {
		t.Fatalf("duplicate requests=%d, want 20", workload.DuplicateRequests)
	}
	if workload.UniqueUsersUsed != 80 {
		t.Fatalf("unique users used=%d, want 80", workload.UniqueUsersUsed)
	}
}

func TestBuildSeckillWorkloadMixedHotTraffic(t *testing.T) {
	workload, err := buildSeckillWorkload(1000, 1000, 20, 95)
	if err != nil {
		t.Fatalf("build workload failed: %v", err)
	}

	if workload.HotRequests != 950 || workload.ColdRequests != 50 {
		t.Fatalf("hot/cold requests=%d/%d, want 950/50", workload.HotRequests, workload.ColdRequests)
	}

	hotSeen := 0
	coldSeen := 0
	for _, req := range workload.Requests {
		switch req.ActivityIndex {
		case 0:
			hotSeen++
		case 1:
			coldSeen++
		default:
			t.Fatalf("unexpected activity index %d", req.ActivityIndex)
		}
	}
	if hotSeen != 950 || coldSeen != 50 {
		t.Fatalf("hot/cold seen=%d/%d, want 950/50", hotSeen, coldSeen)
	}
}

func TestBuildSeckillWorkloadDuplicateRateClampedByUserPool(t *testing.T) {
	workload, err := buildSeckillWorkload(100000, 50000, 20, 100)
	if err != nil {
		t.Fatalf("build workload failed: %v", err)
	}

	if workload.UniqueUsersUsed != 50000 {
		t.Fatalf("unique users used=%d, want 50000", workload.UniqueUsersUsed)
	}
	if workload.DuplicateRequests != 50000 {
		t.Fatalf("duplicate requests=%d, want 50000", workload.DuplicateRequests)
	}
	if workload.EffectiveDuplicatePct != 50 {
		t.Fatalf("effective duplicate pct=%.2f, want 50", workload.EffectiveDuplicatePct)
	}
}

func TestSplitStockByTraffic(t *testing.T) {
	hot, cold := splitStockByTraffic(1000, 95)
	if hot != 950 || cold != 50 {
		t.Fatalf("split stock=%d/%d, want 950/50", hot, cold)
	}
}
