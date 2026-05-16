package main

import "testing"

func TestBuildCheckoutWorkloadDistributesRequestsAcrossProducts(t *testing.T) {
	workload, err := buildCheckoutWorkload(200, 20, 60)
	if err != nil {
		t.Fatalf("build workload failed: %v", err)
	}

	if got := len(workload.Requests); got != 200 {
		t.Fatalf("request count=%d, want 200", got)
	}
	if got := len(workload.RequestsPerItem); got != 20 {
		t.Fatalf("product count=%d, want 20", got)
	}
	if workload.HotProductCount != 4 {
		t.Fatalf("hot product count=%d, want 4", workload.HotProductCount)
	}
	if workload.HotRequests != 120 {
		t.Fatalf("hot requests=%d, want 120", workload.HotRequests)
	}

	totalAssigned := 0
	hotAssigned := 0
	coldAssigned := 0
	for i, count := range workload.RequestsPerItem {
		totalAssigned += count
		if count == 0 {
			t.Fatalf("product %d received no traffic", i)
		}
		if i < workload.HotProductCount {
			hotAssigned += count
		} else {
			coldAssigned += count
		}
	}

	if totalAssigned != 200 {
		t.Fatalf("total assigned=%d, want 200", totalAssigned)
	}
	if hotAssigned != 120 || coldAssigned != 80 {
		t.Fatalf("hot/cold assigned=%d/%d, want 120/80", hotAssigned, coldAssigned)
	}
}

func TestBuildCheckoutWorkloadClampsProductCountToRequestCount(t *testing.T) {
	workload, err := buildCheckoutWorkload(5, 20, 100)
	if err != nil {
		t.Fatalf("build workload failed: %v", err)
	}

	if got := len(workload.Requests); got != 5 {
		t.Fatalf("request count=%d, want 5", got)
	}
	if workload.HotProductCount != 1 {
		t.Fatalf("hot product count=%d, want 1", workload.HotProductCount)
	}
	if got := len(workload.RequestsPerItem); got != 1 {
		t.Fatalf("effective product count=%d, want 1", got)
	}
	if workload.RequestsPerItem[0] != 5 {
		t.Fatalf("request count for only hot product=%d, want 5", workload.RequestsPerItem[0])
	}
	for i, req := range workload.Requests {
		if req.ProductIndex != 0 {
			t.Fatalf("request %d product index=%d, want 0", i, req.ProductIndex)
		}
	}
}
