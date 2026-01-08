package BloomFilter

import (
	"fmt"
	"hash/fnv"
	"math"
)

// 布隆过滤器结构体
type BloomFilter struct {
	m    uint64 // (number of bits)
	k    uint64 // (number of hash functions)
	bits []byte // bit array
}

func NewBloomFilter(expectedN uint64, falsePositiveRate float64) *BloomFilter {
	m, k := estimateParameters(expectedN, falsePositiveRate)
	if m == 0 || k == 0 {
		panic("Invalid parameters for Bloom filter: m or k is zero")
	}
	numBytes := (m + 7) / 8
	return &BloomFilter{
		m:    m,
		k:    k,
		bits: make([]byte, numBytes),
	}
}

// estimateParameters 根据预期的元素数量n和误报率p计算最佳的m和k
// m = - (n * ln(p)) / (ln(2))^2
// k = (m / n) * ln(2)
func estimateParameters(n uint64, p float64) (m uint64, k uint64) {
	if n == 0 || p <= 0 || p >= 1 {
		return 1000, 10
	}
	mFloat := -(float64(n) * math.Log(p)) / (math.Ln2 * math.Ln2)
	kFloat := (mFloat / float64(n)) * math.Ln2

	m = uint64(math.Ceil(mFloat))
	k = uint64(math.Ceil(kFloat))

	if k < 1 {
		k = 1
	}
	return
}
