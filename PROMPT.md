基数估计这一层现在只认稠密表示，稀疏输入也要接住。hllcard 是 Go 1.24 的基数估计库，仅标准库，自检走 scripts/check.sh。

稠密部分已经能用，`sketch_test.go` 全绿；稀疏表示、`Merge`、序列化是这次才要补的。

任务：按 README「对外契约」的 9 条和「本次要补的一层」把它补出来，让既有用例保持全绿、固定件全过。
重点是切换表示前后、以及不同表示互相合并时，`Count` 都必须连续且一致。

验收：
- gofmt -l . 无输出，go build ./... 退出码 0；
- go test ./... 全绿；
- bash scripts/check.sh 退出码 0，9 个场景全过（legacy 2 + sparse 2 + merge 3 + roundtrip 1 + idem 1）。

约束：
1. 不改 `check/`；对外方法名与签名已定死，不要改。
2. `sketch_test.go` 里的既有用例一条都不许删或改。
3. 仅用 Go 标准库，不引入任何第三方依赖。
4. 结果必须确定：不依赖 map 迭代顺序、不依赖时间。
