package hllcard

import (
	"strconv"
	"testing"
)

func addRange(t *testing.T, s *Sketch, prefix string, from, to int) {
	t.Helper()
	for i := from; i < to; i++ {
		s.Add([]byte(prefix + strconv.Itoa(i)))
	}
}

func TestDenseAccuracy(t *testing.T) {
	s, err := New(14)
	if err != nil {
		t.Fatalf("New(14) 出错: %v", err)
	}

	addRange(t, s, "k", 0, 1000)

	got := s.Count()
	if got < 950 || got > 1050 {
		t.Fatalf("Count = %d, 期望 1000±5%%", got)
	}
}

func TestEmptyCount(t *testing.T) {
	s, err := New(14)
	if err != nil {
		t.Fatalf("New(14) 出错: %v", err)
	}

	if got := s.Count(); got != 0 {
		t.Fatalf("空 sketch Count = %d, 期望 0", got)
	}
}

func TestNewPrecisionRange(t *testing.T) {
	if _, err := New(3); err == nil {
		t.Fatalf("New(3) 应返回错误")
	}
	if _, err := New(19); err == nil {
		t.Fatalf("New(19) 应返回错误")
	}
	if _, err := New(14); err != nil {
		t.Fatalf("New(14) 出错: %v", err)
	}
}

func TestAddIdempotent(t *testing.T) {
	s, err := New(14)
	if err != nil {
		t.Fatalf("New(14) 出错: %v", err)
	}

	s.Add([]byte("x"))
	first := s.Count()

	for i := 0; i < 100; i++ {
		s.Add([]byte("x"))
	}

	if got := s.Count(); got != first {
		t.Fatalf("重复加入同一值后 Count = %d, 期望 %d", got, first)
	}
}
