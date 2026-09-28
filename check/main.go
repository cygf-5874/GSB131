// Command check 是 GSB131 hllcard 的固定验收程序。
//
// 用法：
//
//	go run ./check              跑全部场景
//	go run ./check -list        列出场景名
//	go run ./check --only merge 只跑某一组
//
// 输出逐场景 `PASS <组>/<名>` 或 `FAIL <组>/<名>  期望=… 实际=…`，
// 结尾 `结果：通过 x/N`；全过 exit 0，否则 exit 1。失败不早退。
package main

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"hllcard"
)

type assertionError struct {
	expected string
	actual   string
}

func (e *assertionError) Error() string { return "assertion failed" }

func expect(ok bool, expected, actual string) error {
	if ok {
		return nil
	}
	return &assertionError{expected, actual}
}

func mustNew(p uint8) *hllcard.Sketch {
	s, err := hllcard.New(p)
	if err != nil {
		panic(err)
	}
	return s
}

func addRange(s *hllcard.Sketch, prefix string, from, to int) {
	for i := from; i < to; i++ {
		s.Add([]byte(prefix + strconv.Itoa(i)))
	}
}

func build(prefix string, from, to int) *hllcard.Sketch {
	s := mustNew(14)
	addRange(s, prefix, from, to)
	return s
}

func merge2(a, b *hllcard.Sketch) (*hllcard.Sketch, error) {
	if err := a.Merge(b); err != nil {
		return nil, err
	}
	return a, nil
}

func within(got, want uint64, tol float64) bool {
	if want == 0 {
		return got == 0
	}
	return math.Abs(float64(got)-float64(want))/float64(want) <= tol
}

type scenario struct {
	group string
	name  string
	run   func() error
}

func scenarios() []scenario {
	return []scenario{
		{
			group: "legacy",
			name:  "dense-accuracy",
			run: func() error {
				s := mustNew(14)
				addRange(s, "k", 0, 10000)

				got := s.Count()
				return expect(within(got, 10000, 0.02), "≈10000(±2%)", strconv.FormatUint(got, 10))
			},
		},
		{
			group: "legacy",
			name:  "empty-and-invalid-p",
			run: func() error {
				if _, err := hllcard.New(3); err == nil {
					return expect(false, "New(3) 返回错误", "无错误")
				}
				if _, err := hllcard.New(19); err == nil {
					return expect(false, "New(19) 返回错误", "无错误")
				}

				s, err := hllcard.New(14)
				if err != nil {
					return err
				}
				if got := s.Count(); got != 0 {
					return expect(false, "空 Count = 0", strconv.FormatUint(got, 10))
				}

				s2, err := hllcard.New(4)
				if err != nil {
					return err
				}
				if got := s2.Count(); got != 0 {
					return expect(false, "空 Count = 0", strconv.FormatUint(got, 10))
				}
				return nil
			},
		},
		{
			group: "sparse",
			name:  "small-is-sparse",
			run: func() error {
				s := mustNew(14)
				addRange(s, "s", 0, 50)

				if s.Representation() != hllcard.Sparse {
					return expect(false, "50 个元素时是 Sparse", s.Representation().String())
				}
				got := s.Count()
				return expect(got >= 49 && got <= 51, "50±1", strconv.FormatUint(got, 10))
			},
		},
		{
			group: "sparse",
			name:  "switch-to-dense",
			run: func() error {
				s := mustNew(14)

				const total = 30000
				const batch = 10

				var counts []uint64
				var reps []hllcard.Representation
				for n := 0; n < total; n += batch {
					for i := n; i < n+batch && i < total; i++ {
						s.Add([]byte("t" + strconv.Itoa(i)))
					}
					counts = append(counts, s.Count())
					reps = append(reps, s.Representation())
				}

				for i := 1; i < len(counts); i++ {
					if counts[i] < counts[i-1] {
						return expect(false, "Count 单调不减", fmt.Sprintf("第 %d 步下降", i))
					}
				}

				if reps[0] != hllcard.Sparse {
					return expect(false, "起点是 Sparse", reps[0].String())
				}

				saw := false
				var delta float64
				for i := 1; i < len(reps); i++ {
					if reps[i-1] == hllcard.Sparse && reps[i] == hllcard.Dense {
						saw = true
						delta = math.Abs(float64(counts[i])-float64(counts[i-1])) / float64(counts[i-1])
					}
				}
				if !saw {
					return expect(false, "存在 Sparse→Dense 切换", "未观察到切换")
				}
				if delta > 0.01 {
					return expect(false, "切换处相对差 ≤1%", fmt.Sprintf("%.4f", delta))
				}

				last := reps[len(reps)-1]
				if last != hllcard.Dense {
					return expect(false, "最终是 Dense", last.String())
				}
				got := counts[len(counts)-1]
				return expect(within(got, total, 0.02), "≈30000(±2%)", strconv.FormatUint(got, 10))
			},
		},
		{
			group: "merge",
			name:  "commutative-associative",
			run: func() error {
				ab, err := merge2(build("a", 0, 1000), build("b", 0, 1000))
				if err != nil {
					return err
				}
				ba, err := merge2(build("b", 0, 1000), build("a", 0, 1000))
				if err != nil {
					return err
				}
				if ab.Count() != ba.Count() {
					return expect(false, "交换律：Count 相同", fmt.Sprintf("%d vs %d", ab.Count(), ba.Count()))
				}

				left, err := merge2(ab, build("c", 0, 1000))
				if err != nil {
					return err
				}
				bc, err := merge2(build("b", 0, 1000), build("c", 0, 1000))
				if err != nil {
					return err
				}
				right, err := merge2(build("a", 0, 1000), bc)
				if err != nil {
					return err
				}
				if left.Count() != right.Count() {
					return expect(false, "结合律：Count 相同", fmt.Sprintf("%d vs %d", left.Count(), right.Count()))
				}

				union := mustNew(14)
				addRange(union, "a", 0, 1000)
				addRange(union, "b", 0, 1000)
				addRange(union, "c", 0, 1000)
				if left.Count() != union.Count() {
					return expect(false, "并入全集后 Count 相同", fmt.Sprintf("%d vs %d", left.Count(), union.Count()))
				}
				return nil
			},
		},
		{
			group: "merge",
			name:  "cross-representation",
			run: func() error {
				if build("s", 0, 100).Representation() != hllcard.Sparse {
					return expect(false, "稀疏输入是 Sparse", build("s", 0, 100).Representation().String())
				}
				if build("d", 0, 30000).Representation() != hllcard.Dense {
					return expect(false, "稠密输入是 Dense", build("d", 0, 30000).Representation().String())
				}

				sd, err := merge2(build("s", 0, 100), build("d", 0, 30000))
				if err != nil {
					return err
				}
				ds, err := merge2(build("d", 0, 30000), build("s", 0, 100))
				if err != nil {
					return err
				}
				if sd.Count() != ds.Count() {
					return expect(false, "稀疏⊕稠密 与 稠密⊕稀疏 Count 相同", fmt.Sprintf("%d vs %d", sd.Count(), ds.Count()))
				}

				union := mustNew(14)
				addRange(union, "s", 0, 100)
				addRange(union, "d", 0, 30000)
				if sd.Count() != union.Count() {
					return expect(false, "并入全集后 Count 相同", fmt.Sprintf("%d vs %d", sd.Count(), union.Count()))
				}
				return nil
			},
		},
		{
			group: "merge",
			name:  "identity-and-precision",
			run: func() error {
				m, err := merge2(mustNew(14), build("a", 0, 500))
				if err != nil {
					return err
				}
				if m.Count() != build("a", 0, 500).Count() {
					return expect(false, "空⊕a == a", fmt.Sprintf("%d vs %d", m.Count(), build("a", 0, 500).Count()))
				}

				m2, err := merge2(build("a", 0, 500), mustNew(14))
				if err != nil {
					return err
				}
				if m2.Count() != build("a", 0, 500).Count() {
					return expect(false, "a⊕空 == a", fmt.Sprintf("%d vs %d", m2.Count(), build("a", 0, 500).Count()))
				}

				p15, err := hllcard.New(15)
				if err != nil {
					return err
				}
				if err := build("a", 0, 10).Merge(p15); err == nil {
					return expect(false, "不同 p 合并返回错误", "无错误")
				}
				return nil
			},
		},
		{
			group: "roundtrip",
			name:  "stable-and-cross",
			run: func() error {
				dense := build("d", 0, 50000)
				b1, err := dense.MarshalBinary()
				if err != nil {
					return err
				}
				d2 := mustNew(14)
				if err := d2.UnmarshalBinary(b1); err != nil {
					return err
				}
				b2, err := d2.MarshalBinary()
				if err != nil {
					return err
				}
				if !bytes.Equal(b1, b2) {
					return expect(false, "稠密往返逐字节一致", "不一致")
				}
				if d2.Count() != dense.Count() {
					return expect(false, "稠密往返 Count 相同", fmt.Sprintf("%d vs %d", d2.Count(), dense.Count()))
				}

				sp := build("s", 0, 50)
				sb1, err := sp.MarshalBinary()
				if err != nil {
					return err
				}
				sp2 := mustNew(14)
				if err := sp2.UnmarshalBinary(sb1); err != nil {
					return err
				}
				sb2, err := sp2.MarshalBinary()
				if err != nil {
					return err
				}
				if !bytes.Equal(sb1, sb2) {
					return expect(false, "稀疏往返逐字节一致", "不一致")
				}
				if sp2.Count() != sp.Count() {
					return expect(false, "稀疏往返 Count 相同", fmt.Sprintf("%d vs %d", sp2.Count(), sp.Count()))
				}

				sp3 := mustNew(14)
				if err := sp3.UnmarshalBinary(sb1); err != nil {
					return err
				}
				if err := sp3.Merge(build("d", 0, 50000)); err != nil {
					return err
				}
				union := mustNew(14)
				addRange(union, "s", 0, 50)
				addRange(union, "d", 0, 50000)
				if sp3.Count() != union.Count() {
					return expect(false, "反序列化后可继续合并", fmt.Sprintf("%d vs %d", sp3.Count(), union.Count()))
				}
				return nil
			},
		},
		{
			group: "idem",
			name:  "repeat-add",
			run: func() error {
				s := mustNew(14)
				s.Add([]byte("x"))
				first := s.Count()
				for i := 0; i < 1000; i++ {
					s.Add([]byte("x"))
				}
				if s.Count() != first {
					return expect(false, fmt.Sprintf("重复加入后 Count = %d", first), strconv.FormatUint(s.Count(), 10))
				}

				s2 := mustNew(14)
				addRange(s2, "v", 0, 100)
				c2 := s2.Count()
				for r := 0; r < 50; r++ {
					addRange(s2, "v", 0, 100)
				}
				if s2.Count() != c2 {
					return expect(false, fmt.Sprintf("重复整批加入后 Count = %d", c2), strconv.FormatUint(s2.Count(), 10))
				}
				return nil
			},
		},
	}
}

func runScenario(run func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return run()
}

func main() {
	var only string
	list := false

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-list" || args[i] == "--list":
			list = true
		case args[i] == "--only":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--only 缺少取值")
				os.Exit(2)
			}
			i++
			only = args[i]
		case strings.HasPrefix(args[i], "--only="):
			only = strings.TrimPrefix(args[i], "--only=")
		default:
			fmt.Fprintln(os.Stderr, "无法识别的参数："+args[i])
			os.Exit(2)
		}
	}

	all := scenarios()

	if list {
		for _, s := range all {
			fmt.Println(s.group + "/" + s.name)
		}
		os.Exit(0)
	}

	passed, ran := 0, 0
	for _, s := range all {
		if only != "" && s.group != only {
			continue
		}
		ran++
		label := s.group + "/" + s.name

		err := runScenario(s.run)
		if err == nil {
			passed++
			fmt.Println("PASS " + label)
			continue
		}

		expected, actual := "(未抛断言)", err.Error()
		var ae *assertionError
		if errors.As(err, &ae) {
			expected, actual = ae.expected, ae.actual
		}
		fmt.Printf("FAIL %s  期望=%s 实际=%s\n", label, expected, actual)
	}

	fmt.Printf("结果：通过 %d/%d\n", passed, ran)
	if passed != ran {
		os.Exit(1)
	}
}
