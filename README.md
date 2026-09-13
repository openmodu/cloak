# cloak

发给大模型之前脱敏，拿回结果之后还原。

```
原文 ──mask──> __PII_EMAIL_ADDRESS_1__ ──> LLM
                                            │
原文 <──restore── 带占位符的回复 <───────────┘
```

识别并启用脱敏的内容会替换成占位符再发出去，模型回复里的占位符按凭据换回原文。
识别可能漏检；默认构建也不含 NER。还原凭据含敏感原文，不能发送给模型或写入日志。

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
./bin/cloak mask --json -f testdata/en_pii.txt | ./bin/cloak restore

# HTTP 服务
./bin/cloakd --port 8844
curl -s localhost:8844/api/health
```

默认构建是**纯 Go、零系统依赖**，识别只用正则：邮箱、电话、卡号、密码、验证码、
私钥、URL 这些有固定形状的能认出来，人名、机构名、中文完整地址认不出来。
后者需要模型，见「启用 NER」。

## Web workspace

The HTTP server includes an English-language web workspace at `/`:

```bash
go build -o bin/cloakd ./cmd/cloakd
./bin/cloakd --host 127.0.0.1 --port 8844
# Open http://127.0.0.1:8844/
```

For local NER and address masking, install the models with
`bash scripts/setup-ner.sh`, then run:

```bash
bash scripts/run-web.sh --port 18844
# Open http://127.0.0.1:18844/
```

This builds an ONNX-enabled binary and loads `configs/cloak.yaml`, which enables
address masking and address fallback. It uses the runtime in `~/.cloak/lib` by
default; override `CLOAK_ONNXRUNTIME_LIB` for a different installation.

Mask text, copy the result or restoration JSON, and restore the original text.
The **LLM test** tab runs mask → LLM → restore using the server-configured model.
It shows the actual masked prompt, raw model reply, and restored response. Only
clicking **Run LLM test** sends a prompt to the model; Mask and Restore do not.
Each stage shows server-measured elapsed time. The LLM duration includes upstream
network I/O; the total excludes browser transfer time. Successful calls also log
`mask_ms`, `llm_ms`, `restore_ms`, and `total_ms`, without prompt or response text.
Use a fictional example first: detection can miss sensitive details. Model names
can be overridden, but upstream URLs and credentials remain server-controlled.
The page uses the same server APIs. No frontend build,
CDN, external fonts, or browser storage is required. If HTTP authentication is
enabled, enter the token under **Server access**. The static page is public;
API authorization remains enforced. Use HTTPS when accessing a remote server.

Restoration records contain sensitive original details. Keep them private and
copy them before leaving the page; refreshing clears the workspace. Detection
depends on server settings and loaded models, so always review the result.

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

响应使用 `{"output": ..., "error": ...}` 信封（health 返回独立状态对象）。错误为
`{"message":"...","code":null}`，并保留有意义的 HTTP 4xx/5xx 状态码。
`Authorization` 头支持裸 key
与 `Bearer <key>` 两种写法，不配置访问口令时不校验。

还原凭据（`maskMeta`）**服务端不留存**，随响应交给调用方，还原时原样带回来。
注意它里面含有原文，要和脱敏前的文本同等看待——不要写进日志，不要转给第三方。

HTTP 和 CLI JSON 输出使用小端二进制凭据格式，只序列化命中的文本片段；
还原同时接受此格式与旧版的 base64(JSON) 格式。详见 [行为约定](docs/behavior.md)。

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

融合失败时有兜底：如果 NER 给出的地址本身已经含门牌号一类的细节，即便层级链拼
不起来，也保留 NER 的原始边界去脱敏，而不是整条放过。这条兜底是拿真实模型跑出来
的——某些写法（例如种子正好止于「科兴科学园」这类园区名）会让门牌号在层级规整时
被当成倒挂删掉，没有兜底就会让一整条完整地址原样漏出去。

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

另外检查并发安全和带 ONNX 标签的构建：

```bash
CGO_ENABLED=1 go vet -tags cloak_onnx ./...
make test-race
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

### 6. 真实模型（需要先装 NER，见「启用 NER」）

```bash
CLOAK_TEST_MODELS_DIR=~/.cloak/models \
CLOAK_ONNXRUNTIME_LIB=~/.cloak/lib/libonnxruntime.so make test-onnx
```

这条不允许「模型缺失就退化成纯正则」，缺文件直接失败。已经跑通的样本：

| 输入 | 结果 |
|---|---|
| `John Smith works at Microsoft in New York.` | 人名 / 机构 / 地名三项全中，分数 >0.99 |
| `王小明住在北京市朝阳区建国路88号。` | 人名与地址均命中，偏移落在字符边界上 |
| `公司在苏州工业园区星海街星海广场2栋18层1802室办公。` | NER 只给出「苏州」「星海街星海广场」两段碎片，融合补全为完整地址 |
| `请把合同寄到深圳市南山区科技南十二路8-2号科兴科学园C座5层。` | 融合产不出结果，回退到 NER 边界脱敏（见「识别流程」的兜底说明） |
| `我在上海市浦东新区上班，离家很近。` | 只到区县，够不到隐私阈值，按设计不脱敏 |

### 7. 分层约束没有被破坏

```bash
go list -deps ./internal/usecase/... | grep 'cloak/internal/repo' && echo "违规！" || echo "usecase 没有依赖 repo"
```

## 启用 NER

默认只有正则，人名、机构名不会被识别；中文地址融合需要 NER 提供种子区间，
因此也不会生效。

**一键准备模型与运行时**：

```bash
./scripts/setup-ner.sh                                   # 装到 ~/.cloak
HF_ENDPOINT=https://hf-mirror.com ./scripts/setup-ner.sh # 走镜像
CLOAK_HOME=/opt/cloak ./scripts/setup-ner.sh             # 换安装目录
```

脚本可重复执行，已存在的文件跳过，中断的下载续传。装完约 360 MB：
ONNX Runtime 动态库 83 MB，两个模型 276 MB。

**带 ONNX 支持编译并运行**（需要 cgo）：

```bash
CGO_ENABLED=1 go build -tags cloak_onnx -o bin/cloakd ./cmd/cloakd

export CLOAK_ONNXRUNTIME_LIB=~/.cloak/lib/libonnxruntime.so
export CLOAK_MODELS_DIR=~/.cloak/models
export CLOAK_EN_MODEL_ID=Xenova/bert-base-NER
export CLOAK_ZH_MODEL_ID=Xenova/bert-base-multilingual-cased-ner-hrl
./bin/cloakd
```

运行时默认模型：`funstory-ai/neurobert-mini`（英文）和
`ckiplab/bert-tiny-chinese-ner`（中文）。安装脚本选择有现成 ONNX 导出的 Xenova 模型，
须使用脚本输出的模型 ID 环境变量（如上），这不代表两套模型结果相同。可以用 `--en-model` /
`--zh-model`、`CLOAK_EN_MODEL_ID` / `CLOAK_ZH_MODEL_ID` 或配置文件换掉。
换模型时注意标签集须是 `PER` / `ORG` / `LOC` 这一套，否则 `labelToEntityType`
认不出来。

模型目录布局：

```
<models-dir>/<model-id>/config.json                    # 提供 id2label 与 max_position_embeddings
<models-dir>/<model-id>/vocab.txt                      # 或 tokenizer.json
<models-dir>/<model-id>/onnx/model_quantized.onnx
```

模型目录不存在、文件缺失、或二进制没带 ONNX 支持，都只打一条 warning 并退化成
纯正则，不会让服务起不来。

验证真实模型：

```bash
CLOAK_TEST_MODELS_DIR=~/.cloak/models \
CLOAK_TEST_EN_MODEL=Xenova/bert-base-NER \
CLOAK_TEST_ZH_MODEL=Xenova/bert-base-multilingual-cased-ner-hrl \
CLOAK_ONNXRUNTIME_LIB=~/.cloak/lib/libonnxruntime.so make test-onnx
```

要换成远程推理服务，实现 `internal/repo/nerrepo` 的 `Inferencer` 接口即可：

```go
type Inferencer interface {
	Infer(ctx context.Context, inputIDs, attentionMask, tokenTypeIDs []int64) ([][]float32, error)
	Close() error
}
```

## 已知限制

按「会不会让你误判工具坏了」排序。

### 1. `password:` 不会被脱敏，`pwd:` 会

```
my password: hunter2   →  原样输出        规则分数 0.4，低于采纳阈值 0.5
my pwd: hunter2        →  __PII_PASSWORD_1__   规则分数 0.6
```

这是规则表里刻意的取值：`password:` 后面跟的往往是文档、提示语而非真实口令，
分数压在阈值之下以避免误伤。要改就调 `internal/repo/regexrepo/patterns_default.go`
里 `PASSWORD_LITERAL` 的分数，改完跑金样本回归。

### 2. 默认不脱敏物理地址

`maskAddress` 默认 `false`，因为地址极易误伤正常语句。要打开在配置里写
`mask_config.maskAddress: true`，或调 `/api/config`。

### 3. 不开 NER 就没有人名、机构名、中文地址

正则只能认有固定形状的东西。人名、机构名要靠模型；中文地址融合也需要 NER
先给出种子区间，没有 NER 时那套规则一次都不会触发。见「启用 NER」。

### 4. 语言判定是基于字符分布的

先看假名与谚文这类排他性字符定出日文、韩文，再按汉字占比区分中英文，简繁则靠
繁体专用字命中判断。它不是概率语言模型，在中英混排、极短文本上可能判错。
明确知道语言时，HTTP 传 `language` 字段、CLI 用 `--language` 直接指定，能绕开判定。

### 5. 分词器只覆盖 BERT WordPiece

`pkg/tokenize` 是固定的 BasicTokenizer + WordPiece。模型若带 `tokenizer.json`，
其中的 `model.type`、`normalizer`、`pre_tokenizer` 会被校验，**超出实现范围时直接
报错**（随后按「模型不可用」退化成纯正则），不会静默产出错位的 token。
BPE 系模型、ByteLevel/Metaspace 预分词、Precompiled 归一化都不支持。

### 6. 超长文本的尾部不过 NER

按模型 `config.json` 的 `max_position_embeddings` 截断（读不到则 512）。
超出部分不会被 NER 识别，正则仍然扫描全文。

### 7. 简繁转换依赖外部 `opencc` 命令

PATH 里有 `opencc` 时自动接入中文 NER，每次推理批量转换 token；
没装则告警跳过，转换执行失败则中止当前识别。

### 8. 没有浏览器 / WASM 端

Go 编 WASM 在体积与 GC 上都不划算。浏览器侧建议用 JS 单独实现核心逻辑——
那部分本身只有几百行。

## 开发

```bash
make            # fmt + vet + test + build
make test
make wire       # 改了 ProviderSet 之后重新生成注入代码
make clean
```

## License

MIT，见 [LICENSE](LICENSE)。
