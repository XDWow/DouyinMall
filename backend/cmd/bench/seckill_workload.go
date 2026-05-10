package main

import (
	"fmt"
	"math"
	"math/rand"
)

type seckillRequestSpec struct {
	UserID        int64
	ActivityIndex int
}

type seckillWorkload struct {
	Requests               []seckillRequestSpec
	UniqueUsersUsed        int
	DuplicateRequests      int
	EffectiveDuplicatePct  float64
	HotRequests            int
	ColdRequests           int
	RequestedDuplicatePct  int
	RequestedHotTrafficPct int
}

func buildSeckillWorkload(totalRequests, users, duplicatePercent, hotPercent int) (seckillWorkload, error) {
	if totalRequests <= 0 {
		return seckillWorkload{}, fmt.Errorf("requests must be > 0")
	}
	if users <= 0 {
		return seckillWorkload{}, fmt.Errorf("users must be > 0")
	}
	if duplicatePercent < 0 || duplicatePercent > 100 {
		return seckillWorkload{}, fmt.Errorf("duplicate_percent must be within [0,100]")
	}
	if hotPercent < 0 || hotPercent > 100 {
		return seckillWorkload{}, fmt.Errorf("hot_percent must be within [0,100]")
	}

	hotRequests, coldRequests := splitRequestsByPercent(totalRequests, hotPercent)
	uniqueTarget := totalRequests - int(math.Round(float64(totalRequests)*float64(duplicatePercent)/100))
	if uniqueTarget < 1 {
		uniqueTarget = 1
	}
	if uniqueTarget > totalRequests {
		uniqueTarget = totalRequests
	}
	if uniqueTarget > users {
		uniqueTarget = users
	}

	uniqueHot, uniqueCold, err := splitUniqueRequests(uniqueTarget, hotRequests, coldRequests)
	if err != nil {
		return seckillWorkload{}, err
	}

	hotBase := make([]seckillRequestSpec, 0, uniqueHot)
	coldBase := make([]seckillRequestSpec, 0, uniqueCold)
	nextUserID := int64(5_000_000)
	for i := 0; i < uniqueHot; i++ {
		hotBase = append(hotBase, seckillRequestSpec{
			UserID:        nextUserID,
			ActivityIndex: 0,
		})
		nextUserID++
	}
	for i := 0; i < uniqueCold; i++ {
		coldBase = append(coldBase, seckillRequestSpec{
			UserID:        nextUserID,
			ActivityIndex: 1,
		})
		nextUserID++
	}

	specs := make([]seckillRequestSpec, 0, totalRequests)
	specs = append(specs, hotBase...)
	specs = append(specs, coldBase...)

	rng := rand.New(rand.NewSource(20260509))
	appendDuplicates := func(base []seckillRequestSpec, total int) {
		if len(base) == 0 {
			return
		}
		for i := len(base); i < total; i++ {
			specs = append(specs, base[rng.Intn(len(base))])
		}
	}
	appendDuplicates(hotBase, hotRequests)
	appendDuplicates(coldBase, coldRequests)

	rng.Shuffle(len(specs), func(i, j int) {
		specs[i], specs[j] = specs[j], specs[i]
	})

	duplicateRequests := totalRequests - uniqueTarget
	effectiveDuplicatePct := 0.0
	if totalRequests > 0 {
		effectiveDuplicatePct = float64(duplicateRequests) * 100 / float64(totalRequests)
	}

	return seckillWorkload{
		Requests:               specs,
		UniqueUsersUsed:        uniqueTarget,
		DuplicateRequests:      duplicateRequests,
		EffectiveDuplicatePct:  effectiveDuplicatePct,
		HotRequests:            hotRequests,
		ColdRequests:           coldRequests,
		RequestedDuplicatePct:  duplicatePercent,
		RequestedHotTrafficPct: hotPercent,
	}, nil
}

func splitRequestsByPercent(totalRequests, hotPercent int) (hotRequests int, coldRequests int) {
	if hotPercent >= 100 || totalRequests == 1 {
		return totalRequests, 0
	}
	if hotPercent <= 0 {
		return 0, totalRequests
	}

	hotRequests = int(math.Round(float64(totalRequests) * float64(hotPercent) / 100))
	if hotRequests <= 0 {
		hotRequests = 1
	}
	if hotRequests >= totalRequests {
		hotRequests = totalRequests - 1
	}
	return hotRequests, totalRequests - hotRequests
}

func splitUniqueRequests(uniqueTotal, hotRequests, coldRequests int) (uniqueHot int, uniqueCold int, err error) {
	activeActivities := 0
	if hotRequests > 0 {
		activeActivities++
	}
	if coldRequests > 0 {
		activeActivities++
	}
	if activeActivities == 0 {
		return 0, 0, fmt.Errorf("no active activity in workload")
	}
	if uniqueTotal < activeActivities {
		return 0, 0, fmt.Errorf("unique users %d is less than active activities %d", uniqueTotal, activeActivities)
	}
	if coldRequests == 0 {
		return uniqueTotal, 0, nil
	}
	if hotRequests == 0 {
		return 0, uniqueTotal, nil
	}

	uniqueHot = int(math.Round(float64(uniqueTotal) * float64(hotRequests) / float64(hotRequests+coldRequests)))
	if uniqueHot <= 0 {
		uniqueHot = 1
	}
	if uniqueHot >= uniqueTotal {
		uniqueHot = uniqueTotal - 1
	}
	uniqueCold = uniqueTotal - uniqueHot
	return uniqueHot, uniqueCold, nil
}

func splitStockByTraffic(stock, hotPercent int) (hotStock int, coldStock int) {
	if stock <= 0 {
		return 0, 0
	}
	hotRequests, coldRequests := splitRequestsByPercent(stock, hotPercent)
	return hotRequests, coldRequests
}
