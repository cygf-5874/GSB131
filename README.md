# hllcard

HyperLogLog 基数估计库。当前只有稠密表示，本次要接住稀疏输入，并补上合并与序列化。

- 语言/依赖：Go 1.24，仅标准库，无第三方依赖。
- 构建：`bash scripts/build.sh`（即 `go build ./...`）。
- 自检：`bash scripts/check.sh`（`check/` 是**固定验收程序，勿改**）。
- 既有用例：`go test ./...`。

## 用法

```go
s, _ := hllcard.New(14)
s.Add([]byte("a"))
s.Count()
```

## 对外契约

1. 既有稠密 `Add` / `Count` 行为不得回归。
2. 新增**稀疏表示**，超过阈值自动转稠密；**切换前后 `Count` 连续**
   （同一数据集在切换点两侧的估计值相对差 ≤ 1%）。
3. 新增 `Merge` 满足**交换律与结合律**；稀疏/稠密任意组合都对。
4. `MarshalBinary` / `UnmarshalBinary` 往返**逐字节确定**，且**跨表示可互认**
   （反序列化出来的 sketch 可以继续与另一种表示合并）。
5. 哈希与种子固定、确定；**不得**依赖 map 迭代顺序。
6. `p=14` 时相对误差 ≤ 2%。
7. `Add` **幂等**：重复加入同一值不改变计数。
8. 空 sketch 参与合并等于另一侧（`empty ⊕ a == a`）。
9. 不同 `p` 之间 `Merge` 抛错（返回错误）。

## 本次要补的一层

- **稀疏表示**：小基数时用稀疏结构保存「非零桶」；当非零桶数超过 `sparseLimit = 4096`
  时自动切换成稠密表示（`m = 2^p` 个桶）。两种表示必须给出同一个 `Count`。
- `Representation() Representation`：如实报告当前表示（`Sparse` / `Dense`）。
  现在恒返回 `Dense`，实现稀疏后要按实际表示返回。
- `Merge(other *Sketch) error`：逐桶取最大值合并；`p` 不一致返回错误。
- `MarshalBinary` / `UnmarshalBinary`：格式自定，但必须确定且往返一致。

## 目录

```
sketch.go        库实现（稠密部分已可用；稀疏 / Merge / Marshal 待补）
sketch_test.go   既有用例
check/main.go    固定验收程序（9 个场景，勿改）
scripts/build.sh 构建
scripts/check.sh 自检入口
```
