# cloak

发给大模型之前脱敏，拿回结果之后还原。

```
原文 ──mask──> __PII_EMAIL_ADDRESS_1__ ──> LLM
                                            │
原文 <──restore── 带占位符的回复 <───────────┘
```

敏感内容永远不离开本机：替换成占位符再发出去，模型回复里的占位符再按凭据换回原文。
凭据只存位置，不复制内容。

## 保护范围

12 类敏感实体：

| 类别 | 实体 |
|---|---|
| 身份 | 人名、机构名、物理地址 |
| 联系方式 | 邮箱、电话、URL |
| 金融 | 银行卡号、支付信息 |
| 凭证 | 密码、验证码、私钥、随机种子 |

物理地址默认**不**脱敏（误伤正常语句的概率高，需要显式打开），其余默认全开。

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

默认构建是**纯 Go、零系统依赖**，识别只用正则：邮箱、电话、卡号、密码、验证码、
私钥、URL 这些有固定形状的能认出来，人名、机构名、中文完整地址认不出来。
后者需要模型，见「启用 NER」。

## HTTP 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/health` | 健康检查，免鉴权 |
| POST | `/api/call` | 完整链路：脱敏 → 调模型 → 还原 |
| POST | `/api/mask_text` | 脱敏，返回脱敏文本与还原凭据 |
| POST | `/api/restore_text` | 凭还原凭据把占位符换回原文 |
| POST | `/api/mask_text_batch` | 批量脱敏 |
| POST | `/api/restore_text_batch` | 批量还原 |
| POST | `/api/config` | 免重启修改脱敏开关 |

响应统一是 `{"output": ..., "error": ...}` 信封。`Authorization` 头支持裸 key
与 `Bearer <key>` 两种写法，不配置访问口令时不校验。

还原凭据（`maskMeta`）**服务端不留存**，随响应交给调用方，还原时原样带回来。
注意它里面含有原文，要和脱敏前的文本同等看待——不要写进日志，不要转给第三方。

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

**usecase 永不 import repo**，只认 `internal/usecase/port.go` 里的 4 个接口：

```go
type Recognizer interface { ... }   // 正则、本地 NER、远程 NER 服务都是它的实现
type LangDetector interface { ... } // 语言判定，决定是否启用中文地址规则
type ConfigStore interface { ... }  // 可热更新的脱敏开关
type LLMClient interface { ... }    // 下游大模型
```

`internal/usecase/masker/masker_test.go` 全程用 stub 识别器，一行 repo 都不碰——
这条约束是被测试实际验证着的（见「怎么测试」第 6 条）。

改了 `internal/bootstrap/provider.go` 里的 `ProviderSet` 之后：

```bash
make wire     # 重新生成 internal/bootstrap/wire_gen.go
```

## 识别流程

```
文本
 ├─ 正则识别器     每个实体类型一组规则，独立扫全文
 └─ NER 识别器     模型逐字打 BIO 标签，聚合成实体区间（可选）
        ↓
   中文地址融合     仅中文：把 NER 切碎的地址片段按层级链重新粘成完整地址
        ↓
   区间规整         按分数过滤(≥0.5) → 同范围去重 → 重叠消解(分数高>跨度长>起点早)
        ↓
   占位符替换       __PII_<TYPE>_<序号>__，同时记下位置
```

中文地址融合按「国家 → 省 → 市 → 区县 → 乡镇 → 道路 → 门牌号 → POI → 楼栋 →
楼层 → 房间」11 级层级链工作，只有细到门牌号，或同时有 POI 与楼层/房间，才算
能定位到具体住户的敏感地址——只到「上海市浦东新区」这种粒度不脱敏。

## 配置

取值优先级：**命令行 > 环境变量 > 配置文件 > 默认值**。
环境变量前缀 `CLOAK_`，例如 `CLOAK_PORT`、`CLOAK_API_KEY_FILE`、`CLOAK_MODELS_DIR`。
配置文件见 `configs/cloak.yaml`。

LLM API key 文件格式：

```json
{
  "openai-api-key": "your-key",
  "openai-base-url": "https://api.openai.com/v1",
  "openai-model": "gpt-4o-mini"
}
```

任何 OpenAI 兼容的端点都能用，换厂商改 `openai-base-url` 即可。

## 怎么测试

### 1. 自动化检查

```bash
make            # fmt + vet + test + build
```

84 个测试，13 个包。另外确认带 ONNX 的构建也是通的：

```bash
CGO_ENABLED=1 go vet -tags cloak_onnx ./...
```

### 2. 金样本回归

`testdata/` 下有中英文两份覆盖全部实体类型的样本，以及它们的期望脱敏结果。
任何改动让输出偏离一个字节，测试就会失败：

```bash
go test ./internal/bootstrap/ -run TestMaskMatchesGolden -v
```

手工看 diff：

```bash
./bin/cloak mask -f testdata/en_pii.txt | diff - testdata/en_pii.masked.golden.txt
./bin/cloak mask -f testdata/zh_pii.txt | diff - testdata/zh_pii.masked.golden.txt
```

中文地址融合另有一份 60 条地址的数据集与金样本：

```bash
go test ./pkg/zhaddr/                      # 比对
go test ./pkg/zhaddr/ -update              # 规则调整后重新生成金样本
```

### 3. 往返一致性

脱敏再还原必须逐字节等于原文：

```bash
./bin/cloak roundtrip -f testdata/en_pii.txt
./bin/cloak roundtrip -f testdata/zh_pii.txt
```

### 4. 拿你自己的文本试

```bash
./bin/cloak spans -f /path/to/your.txt     # 看认出了什么、有没有误伤
echo "联系 zhang@corp.com" | ./bin/cloak mask
```

### 5. 用假 LLM 验证完整链路 ← 不花真 key

`/api/call` 光看返回值看不出中间到底送出去了什么。`scripts/fakellm` 是个假的
OpenAI 兼容端点：它把收到的 prompt 打到终端，再原样当回复返回。于是你能
**亲眼看到真正离开本机的内容**，同时验证还原后逐字等于原文。

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

终端 3 拿到原文完整还原：

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

### 6. 分层约束没有被破坏

```bash
go list -deps ./internal/usecase/... | grep 'cloak/internal/repo' && echo "违规！" || echo "usecase 没有依赖 repo"
```

## 启用 NER

默认只有正则，人名、机构名不会被识别；中文地址融合需要 NER 提供种子区间，
因此也不会生效。启用需要两样东西。

**一、带 ONNX 支持编译**（需要 cgo 与 ONNX Runtime 动态库）：

```bash
CGO_ENABLED=1 go build -tags cloak_onnx -o bin/cloakd ./cmd/cloakd
export CLOAK_ONNXRUNTIME_LIB=/path/to/libonnxruntime.so
```

动态库从 https://github.com/microsoft/onnxruntime/releases 下对应平台的包即可。

**二、准备模型目录**，布局如下：

```
<models-dir>/funstory-ai/neurobert-mini/config.json
<models-dir>/funstory-ai/neurobert-mini/vocab.txt          # 或 tokenizer.json
<models-dir>/funstory-ai/neurobert-mini/onnx/model_quantized.onnx
<models-dir>/ckiplab/bert-tiny-chinese-ner/...             # 中文同上
```

这两个是默认模型（英文 / 中文）。模型从 HuggingFace 下载；仓库里没有现成 ONNX
的话用 `optimum-cli export onnx` 自行导出并量化。然后：

```bash
./bin/cloakd --models-dir /path/to/ner-models
# 或 CLOAK_MODELS_DIR
```

模型目录不存在、文件缺失、或二进制没带 ONNX 支持，都只打一条 warning 并退化成
纯正则，不会让服务起不来。

要换模型或接远程推理服务，实现 `internal/repo/nerrepo` 的 `Inferencer` 接口即可：

```go
type Inferencer interface {
	Infer(ctx context.Context, inputIDs, attentionMask, tokenTypeIDs []int64) ([][]float32, error)
	Close() error
}
```

## 已知限制

1. **真实 ONNX 推理尚未端到端验证过**。推理器接口、分词、偏移定位、BIO 聚合、
   标签映射全部有测试覆盖（用假推理器），但真实模型那一步需要你自己补一次验证。
2. **简繁转换未接入**。繁体模型配简体输入时，理想做法是简→繁喂模型、繁→简还原
   token。`nerrepo.TextConverter` 接口已经留好，默认为 nil（不转换）。
3. **没有浏览器 / WASM 端**。Go 编 WASM 在体积与 GC 上都不划算，浏览器侧建议
   用 JS 单独实现核心逻辑——那部分本身只有几百行。

## 开发

```bash
make            # fmt + vet + test + build
make test
make wire       # 改了 ProviderSet 之后重新生成注入代码
make clean
```

## License

MIT，见 [LICENSE](LICENSE)。
