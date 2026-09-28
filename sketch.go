// Package hllcard 提供 HyperLogLog 基数估计（当前只有稠密表示）。
//
// 估计器按固定哈希工作：同一输入多次加入不会改变计数；不同进程、
// 不同机器上的同一数据集得到同一个估计值，不依赖 map 迭代顺序。
package hllcard

import (
	"errors"
	"hash/fnv"
	"math"
	"math/bits"
)

// 精度参数 p 的合法范围，桶数 m = 2^p。
const (
	MinP     = 4
	MaxP     = 18
	DefaultP = 14
)

var (
	// ErrInvalidP 表示精度参数越界。
	ErrInvalidP = errors.New("hllcard: precision out of range")
	// ErrPrecisionMismatch 表示两个 sketch 的精度参数不一致。
	ErrPrecisionMismatch = errors.New("hllcard: precision mismatch")
)

// Representation 是 sketch 的内部表示类别。
type Representation int

const (
	// Sparse 是稀疏表示（用于小基数）。
	Sparse Representation = iota
	// Dense 是稠密表示（用于大基数）。
	Dense
)

func (r Representation) String() string {
	switch r {
	case Sparse:
		return "sparse"
	case Dense:
		return "dense"
	default:
		return "unknown"
	}
}

// Sketch 是一个 HyperLogLog 基数估计器。
type Sketch struct {
	p    uint8
	m    uint32
	regs []uint8
}

// New 新建一个精度为 p 的空 sketch。
func New(p uint8) (*Sketch, error) {
	if p < MinP || p > MaxP {
		return nil, ErrInvalidP
	}
	m := uint32(1) << p
	return &Sketch{p: p, m: m, regs: make([]uint8, m)}, nil
}

// P 返回精度参数。
func (s *Sketch) P() uint8 { return s.p }

// Add 把一个元素加入 sketch。
func (s *Sketch) Add(v []byte) {
	h := hash64(v)
	idx := h >> (64 - s.p)
	rest := h << s.p
	rank := uint8(bits.LeadingZeros64(rest)) + 1
	if max := uint8(64-s.p) + 1; rank > max {
		rank = max
	}
	if rank > s.regs[idx] {
		s.regs[idx] = rank
	}
}

// Count 返回当前基数的估计值。
func (s *Sketch) Count() uint64 {
	mf := float64(s.m)
	var sum float64
	zeros := 0
	for _, r := range s.regs {
		sum += 1.0 / float64(uint64(1)<<uint(r))
		if r == 0 {
			zeros++
		}
	}

	var alpha float64
	switch s.m {
	case 16:
		alpha = 0.673
	case 32:
		alpha = 0.697
	case 64:
		alpha = 0.709
	default:
		alpha = 0.7213 / (1 + 1.079/mf)
	}

	est := alpha * mf * mf / sum
	if est <= 2.5*mf && zeros > 0 {
		est = mf * math.Log(mf/float64(zeros))
	}
	if est < 0 {
		est = 0
	}
	return uint64(est + 0.5)
}

// Representation 报告当前内部表示。
func (s *Sketch) Representation() Representation { return Dense }

// Merge 把 other 的基数并入当前 sketch。
func (s *Sketch) Merge(other *Sketch) error {
	return errors.New("hllcard: Merge not implemented")
}

// MarshalBinary 把 sketch 序列化成字节。
func (s *Sketch) MarshalBinary() ([]byte, error) {
	return nil, errors.New("hllcard: MarshalBinary not implemented")
}

// UnmarshalBinary 从字节还原 sketch。
func (s *Sketch) UnmarshalBinary(data []byte) error {
	return errors.New("hllcard: UnmarshalBinary not implemented")
}

func hash64(v []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(v)
	x := h.Sum64()
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	x *= 0xc4ceb9fe1a85ec53
	x ^= x >> 33
	return x
}
