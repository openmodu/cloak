# cloak

发给大模型之前脱敏，拿回结果之后还原。Go 实现，复刻自 [aifw](https://github.com/funstory-ai/aifw)（Zig + Rust 核心）。

```
原文 ──mask──> __PII_EMAIL_ADDRESS_1__ ──> LLM
                                            │
原文 <──restore── 带占位符的回复 <───────────┘
```

保护 12 类敏感实体：物理地址、邮箱、机构名、人名、电话、银行卡号、支付信息、
验证码、密码、随机种子、私钥、URL。

## 快速开始

```bash
make build

# CLI
./bin/cloak roundtrip -f testdata/en_pii.txt   # 脱敏 + 还原并校验往返一致
./bin/cloak mask      -f testdata/zh_pii.txt   # 只输出脱敏文本
./bin/cloak spans     -f testdata/en_pii.txt   # 以 JSON 查看被脱敏的区间

# HTTP 服务
./bin/cloakd --port 8844
curl -s localhost:8844/api/health
```

默认构建是**纯 Go、零系统依赖**：识别只用正则。要启用 NER（人名、机构名、中文地址）
见下面「启用 NER」。

## 工程约定

| 目录 | 职责 |
|---|---|
| `cmd/` | 可执行入口，只做启动 |
| `internal/types/` | 贫血类型：实体、区间、还原凭据、脱敏开关 |
| `internal/usecase/` | 业务用例。`port.go` 声明全部外部依赖接口 |
| `internal/repo/` | 底层原子能力，实现 usecase 声明的接口 |
| `internal/transport/` | 协议适配（CLI / HTTP），不含业务判断 |
| `internal/bootstrap/` | 装配，由 google/wire 在编译期生成注入代码 |
| `pkg/` | 与业务无关的基础模块，可被外部直接引用 |

依赖方向只有一条：`transport → usecase ← repo`，`usecase → pkg`。
**usecase 永不 import repo**，只认 `internal/usecase/port.go` 里的 5 个接口
（`Recognizer` / `LangDetector` / `ConfigStore` / `LLMClient`）。
`internal/usecase/masker/masker_test.go` 全程用 stub 识别器，一行 repo 都不碰——
这条约束是被测试实际验证着的。

改了 `ProviderSet` 之后跑 `make wire` 重新生成 `internal/bootstrap/wire_gen.go`。

## 复刻原则

**行为以上游为准。** 遇到上游做法看起来可以改进的地方，照抄上游，把理由写进注释，
不擅自"优化"。已按此原则对齐的地方：

| 点 | 上游做法 | 出处 |
|---|---|---|
| 识别器组织 | 一个 EntityType 一个 RegexRecognizer，按枚举顺序建出一整组 | `aifw_core.zig:793` |
| 正则扫描 | 从 pos 起在**剩余文本切片**上找下一个匹配，取捕获组后把 pos 推到组尾 | `RegexRecognizer.zig:run` |
| 二次校验 | `ValidateResultFn` 返回 `?f32`，`orelse` 默认分——只调分数，从不否决匹配 | `RegexRecognizer.zig:run` |
| 词边界 | Rust regex-automata 开了 unicode feature，`\b` 是 **Unicode** 词边界 | `libs/regex/Cargo.toml` |
| 还原 | 由凭据驱动：逐条重新生成占位符，找它**第一次出现**的位置，排序后重建 | `aifw_core.zig:693` |
| 还原凭据 | 服务端不留存，随响应回传给调用方，还原时带回来 | `libaifw.py:8` |
| 采纳阈值 | 写死 0.5 | `aifw_core.zig:MaskPipeline.run` |
| 开关位图 | 地址第 0 位、邮箱第 1 位……默认除地址外全开 | `aifw_core.zig:262` |
| 地址隐私阈值 | 必须有门牌号(L5)，或 POI(L4) + 楼层/房间(L2/L1) | `merge_zh_addr.zig:1058` |
| 模型缺失 | 打一条 warning 退化成纯正则，不让服务起不来 | `libner.py:build_ner_pipeline` |

由此保留的上游行为（是复刻，不是笔误）：

- 同一个占位符在回复里重复出现时，**只还原第一处**。
- `Masker.Spans` 走的是完整脱敏流程，因此**受脱敏开关影响**——被关掉的类型不出现在结果里。
- `\b\d{4,8}\b` 在「A座1208室」里不匹配 1208，因为汉字算 Unicode 词字符。

## 与上游的模块对照

| aifw | cloak |
|---|---|
| `libs/regex`（Rust 165 行） | **删掉**，标准库 `regexp` + 编译期词边界翻译 |
| `core/RegexRecognizer.zig` | `internal/repo/regexrepo/` |
| `core/NerRecognizer.zig` BIO 聚合 | `internal/repo/nerrepo/aggregate.go` |
| `core/SpanMerger.zig` + `dedupOverlappingSpans` | `pkg/span/` |
| `core/merge_zh_addr.zig`（1093 行） | `pkg/zhaddr/` |
| `MaskPipeline` / `RestorePipeline` | `internal/usecase/masker` / `internal/usecase/restorer` |
| `Session` + mask 位图 | `internal/types/maskconfig.go` + `ConfigStore` |
| `libs/aifw-py/libner.py` | `pkg/tokenize/` + `pkg/onnxrt/` + `internal/repo/nerrepo/` |
| `libaifw.py` 的 `detect_language` | `pkg/textutil/` + `internal/repo/langrepo/` |
| `cli/python/aifw.py`（963 行） | `cmd/cloak` + `internal/transport/cli` |
| `py-origin/services` | `cmd/cloakd` + `internal/transport/http` |

## 怎么验收

### 1. 一条命令过全部自动化检查

```bash
make            # fmt + vet + test + build
```

84 个测试，13 个包。另外确认带 ONNX 的构建也是通的：

```bash
CGO_ENABLED=1 go vet -tags cloak_onnx ./...
```

### 2. 逐字节对齐上游金样本（最硬的证据）

`testdata/en_pii.masked.golden.txt` 与 `zh_pii.masked.golden.txt` 直接取自上游仓库的
`tests/*.anonymized.expected.txt`。这两份样本产出时 NER 未参与（人名、公司名、中文地址
都没被脱敏），正好是纯正则路径的预期输出。

```bash
go test ./internal/bootstrap/ -run TestMaskMatchesUpstreamGolden -v
```

想手工看：

```bash
./bin/cloak mask -f testdata/en_pii.txt | diff - testdata/en_pii.masked.golden.txt && echo 一致
./bin/cloak mask -f testdata/zh_pii.txt | diff - testdata/zh_pii.masked.golden.txt && echo 一致
```

### 3. 中文地址融合与上游 Zig 实现对拍

`core/merge_zh_addr.zig` 只依赖 `recog_entity.zig`，不需要 Rust 正则库，可以单独编译。
脚本会把它编出来，用同一份数据集、同样的种子跑一遍，再和本仓库的金样本 diff：

```bash
make crosscheck                          # 默认找 ../aifw
./scripts/crosscheck-zhaddr.sh /path/to/aifw
```

需要 zig 0.15.x 在 PATH 里，或 `ZIG=/path/to/zig`
（`mise install zig@0.15.2`，或从 https://ziglang.org/download/ 直接下）。

**这一项已经跑通**：`testdata/zh_address_dataset.txt` 全部 60 行，
分数、字节偏移、文本与上游 Zig 实现逐行一致。`testdata/zh_address.golden.txt`
因此不是"本实现的当前行为快照"，而是经上游校验过的期望输出。

### 4. 往返一致性

脱敏再还原必须逐字节等于原文：

```bash
./bin/cloak roundtrip -f testdata/en_pii.txt
./bin/cloak roundtrip -f testdata/zh_pii.txt
```

### 5. HTTP 接口

```bash
./bin/cloakd --port 8844 &

curl -s localhost:8844/api/health
# {"status":"ok"}

curl -s -X POST localhost:8844/api/mask_text -H 'Content-Type: application/json' \
  -d '{"text":"My email is test@example.com and my phone is 18744325579.","language":"en"}'
# {"output":{"text":"My email is __PII_EMAIL_ADDRESS_1__ and my phone is __PII_PHONE_NUMBER_2__.","maskMeta":"..."},"error":null}

# 把上一步的 maskMeta 原样带回来
curl -s -X POST localhost:8844/api/restore_text -H 'Content-Type: application/json' \
  -d '{"text":"<上一步的 text>","maskMeta":"<上一步的 maskMeta>"}'

# 免重启改开关
curl -s -X POST localhost:8844/api/config -H 'Content-Type: application/json' \
  -d '{"maskConfig":{"maskEmail":false}}'
```

接口清单与报文格式对齐上游 `docs/oneaifw_services_api.md`：
`/api/health`、`/api/call`、`/api/config`、`/api/mask_text`、`/api/restore_text`、
`/api/mask_text_batch`、`/api/restore_text_batch`。

### 6. 用假 LLM 验证完整链路 ← 不花真 key

`/api/call` 是「脱敏 → 调模型 → 还原」的全链路，光看返回值看不出中间到底送出去了什么。
`scripts/fakellm` 是个假的 OpenAI 兼容端点：它把收到的 prompt 打到终端，再原样当回复返回。
于是你能**亲眼看到真正离开本机的内容**，同时验证还原后逐字等于原文。

```bash
# 终端 1：假 LLM
go run ./scripts/fakellm --port 18080

# 终端 2：指向假 LLM 启动服务
cat > /tmp/fake-key.json <<'JSON'
{
  "openai-api-key": "not-a-real-key",
  "openai-base-url": "http://127.0.0.1:18080/v1",
  "openai-model": "fake-model"
}
JSON
./bin/cloakd --port 8844 --api-key-file /tmp/fake-key.json

# 终端 3：发一条带敏感信息的文本
curl -s -X POST localhost:8844/api/call -H 'Content-Type: application/json' \
  -d '{"text":"我的邮箱是 test@example.com，电话 18744325579，密码为 pwd: S3cure!Pass"}'
```

返回（原文完整还原）：

```json
{"output":{"text":"我的邮箱是 test@example.com，电话 18744325579，密码为 pwd: S3cure!Pass"},"error":null}
```

终端 1 同时打出真正发出去的内容：

```
===== 真正发给「大模型」的内容 =====
我的邮箱是 __PII_EMAIL_ADDRESS_1__，电话 __PII_PHONE_NUMBER_2__，密码为 pwd: __PII_PASSWORD_3__
===== 结束 =====
```

确认无误后把 `openai-base-url` / `openai-api-key` 换成真实的即可。

### 7. 分层约束没有被破坏

```bash
go list -deps ./internal/usecase/... | grep 'cloak/internal/repo' && echo "违规！" || echo "usecase 没有依赖 repo"
```

## 启用 NER

默认只有正则，人名、机构名、中文地址不会被识别（中文地址融合需要 NER 提供种子区间）。
启用需要两样东西：

**一、带 ONNX 支持编译**（需要 cgo 与 ONNX Runtime 动态库）：

```bash
CGO_ENABLED=1 go build -tags cloak_onnx -o bin/cloakd ./cmd/cloakd
export CLOAK_ONNXRUNTIME_LIB=/path/to/libonnxruntime.so
```

**二、按上游的目录布局准备模型**：

```
<models-dir>/funstory-ai/neurobert-mini/config.json
<models-dir>/funstory-ai/neurobert-mini/vocab.txt          # 或 tokenizer.json
<models-dir>/funstory-ai/neurobert-mini/onnx/model_quantized.onnx
<models-dir>/ckiplab/bert-tiny-chinese-ner/...             # 中文同上
```

模型可以用上游 `tools/fetch_hf_models.py` 或 `tests/transformer-js/scripts/prep-models.mjs` 准备。
然后：

```bash
./bin/cloakd --models-dir /path/to/ner-models
# 或 CLOAK_MODELS_DIR / AIFW_MODELS_DIR
```

任何一步不成立都只打一条 warning 并退化成纯正则——与上游模型缺失时的行为一致。

## 配置

取值优先级：**命令行 > 环境变量 > 配置文件 > 默认值**，与上游一致。
环境变量同时认 `CLOAK_` 与 `AIFW_` 两个前缀，方便直接复用上游的环境。
配置文件见 `configs/cloak.yaml`，字段名沿用上游 `assets/aifw.yaml`。

## 已知差异

这几处与上游不同，都是权衡后的选择，不是疏漏：

1. **正则引擎**：上游为了在 WASM 里跑，用 Rust `regex-automata` 编了个 C ABI 静态库；
   这里用标准库 `regexp`，并在编译期把 `\b` 翻译成 RE2 可表达的 Unicode 词边界写法
   （见 `internal/repo/regexrepo/wordboundary.go`）。规则表里的表达式与上游逐字节一致。
2. **偏移全程按字节**：上游 Python 绑定按码点算完再转字节偏移，Go 的字符串本身就是
   字节序列，那层转换不存在。大小写转换会改变字节长度的字符（`İ`、`K`）另做了索引映射，
   否则偏移会整体错位。
3. **`maskMeta` 编码**：上游 core 是二进制布局，这里是 `base64(JSON)`。对调用方同样不透明。
   注意它**含有原文**，要和脱敏前的文本同等看待，不要写进日志或转给第三方——上游同此。
4. **简繁转换未接入**：上游用 OpenCC 做简→繁喂模型、繁→简还原 token。这里留了
   `nerrepo.TextConverter` 接口，默认为 nil（不转换）。上游在没装 OpenCC 时也是跳过。
5. **`pkg/onnxrt` 无法在无模型环境下端到端验证**：推理器接口、分词、定位、聚合、
   标签映射全部有测试覆盖（用假推理器），但真实模型推理这一步需要你自己补一次验证。

## 当前进度

已完成：正则识别、区间规整、占位符与还原、脱敏开关、中文地址融合、NER 流水线、
LLM 代理、CLI、HTTP 服务、wire 装配。

未做：浏览器/WASM 端（Go 编 WASM 体积与 GC 都不划算，那条线建议保留上游的 Zig wasm
或用 JS 重写核心逻辑，核心逻辑本身只有几百行）。
