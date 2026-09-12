# 行为约定

一些不看代码就不容易知道的实现决定。反直觉的行为在 README 的「已知限制」里，
这里记的是各项配置与接口的确切语义。

| 项目 | 行为 |
|---|---|
| 脱敏配置 | YAML `mask_config` 启动时应用，`maskAll` 先执行、单项覆盖；HTTP 可热更新 |
| 默认温度 | `--temperature` > `CLOAK_TEMPERATURE` > YAML；请求显式传 0 会覆盖默认值 |
| 服务日志 | `--log-level` > `CLOAK_LOG_LEVEL` > YAML |
| 路径类配置 | `models_dir`、`api_key_file` 开头的 `~` 会展开成用户主目录 |
| 请求级 apiKeyFile | 默认关闭；配置 `--api-key-dir` 后才启用，且只接受该目录内的相对路径 |
| 错误响应 | `{message, code}` 对象，配合 HTTP 4xx/5xx 状态码 |
| 自动语言 | 省略、空字符串、`auto` 均触发自动检测；假名/谚文占字母与汉字至少 25% 才判日韩，再按汉字占比定中英，简繁靠繁体专用字；这是启发式，不等同于 aifw 的 langdetect，混合文本可显式指定语言 |
| 凭据 | HTTP/CLI JSON 输出小端二进制 ABI，只序列化命中的文本片段；读取同时兼容旧的 base64(JSON) |
| Unicode 正则 | 内置规则的数字、空白及词边界使用 Unicode 集合；自定义规则中的复杂嵌套/否定类不保证与其他正则引擎语义一致 |
| NER 分词 | 共享转换器串行保护，有并发回归测试；`tokenizer.json` 声明的流水线超出实现范围时加载即失败 |
| NER 序列长度 | 取模型 `config.json` 的 `max_position_embeddings`，读不到退回 512 |
| 地址融合兜底 | 默认遵循 aifw，移除原始地址种子；显式设置 YAML `address_fallback: true` 才在融合失败时保留达到隐私阈值的 NER 原始地址 |
| 简繁转换 | 安装 OpenCC 后中文 NER 自动接入；繁体输入不转换，简体输入 s2t，token 批量 t2s |

## CLI

```bash
cloak mask --json -f input.txt > masked.json
cloak restore -f masked.json
cloak mask-batch -f requests.json
cloak restore-batch -f masked-batch.json
cloak call --api-key-file keys.json --config configs/cloak.yaml -f input.txt
```

`mask-batch` 输入 `[{"text":"...","language":"auto"}]`；还原输入含 `text` 和
`maskMeta`。`--config`、`--models-dir`、`--language` 可用于本地识别。
旧的 `mask` 文本输出不变，需保存凭据时加 `--json`。

## 验证边界

`make test` 包括规则金样本、并发归一化、配置、凭据 ABI 固定样本、HTTP 参数和 CLI
往返回归。`make test-race` 运行 race detector。OpenCC 测试在缺少可执行文件时跳过。

分词器只接受实现支持的 BERT 流水线；NFKC 等额外归一化及 WhitespaceSplit 等
不同预分词会在加载时明确拒绝。序列长度取模型位置上限与 tokenizer 上限的较小值，
调用方只能进一步缩短。密钥文件使用固定目录句柄打开，避免检查后路径被替换导致越界。
下载使用临时文件和原子替换，完成后记录 URL 与本地 SHA-256；无标记或损坏文件会重下。
此校验用于发现本地损坏，不是远端签名验证，也不固定远端 main 的版本。

真实模型检查需要提供两个模型目录及 ONNX Runtime 动态库：

```bash
CLOAK_TEST_MODELS_DIR=/path/to/models \
CLOAK_ONNXRUNTIME_LIB=/path/to/libonnxruntime.so make test-onnx
```

运行时默认模型是 aifw 的 `funstory-ai/neurobert-mini`（英文）与
`ckiplab/bert-tiny-chinese-ner`（中文），可用 `--en-model` /
`--zh-model`、`CLOAK_EN_MODEL_ID` / `CLOAK_ZH_MODEL_ID` 或 YAML 覆盖。
标签集须是 PER / ORG / LOC 这一套。

以下历史验证使用 Xenova/bert-base-NER 与 Xenova/bert-base-multilingual-cased-ner-hrl（ONNX Runtime 1.30.0，CPU），不是对原始默认模型的验证；运行测试时须用 `CLOAK_TEST_EN_MODEL` / `CLOAK_TEST_ZH_MODEL` 显式选择这两个 ID：

| 样本 | 结果 |
|---|---|
| `John Smith works at Microsoft in New York.` | 人名 / 机构 / 地名三项全中，分数 >0.99 |
| `王小明住在北京市朝阳区建国路88号。` | 人名与地址均命中，偏移落在字符边界上 |
| `公司在苏州工业园区星海街星海广场2栋18层1802室办公。` | NER 只给出「苏州」与「星海街星海广场」两段碎片，地址融合补全为完整地址 |
| `请把合同寄到深圳市南山区科技南十二路8-2号科兴科学园C座5层。` | 融合产不出结果；仅在 `address_fallback: true` 时回退到 NER 边界脱敏 |
| `我在上海市浦东新区上班，离家很近。` | 只到区县，够不到隐私阈值，按设计不脱敏 |

这是冒烟测试，确认整条链路在真实模型上跑得通，不是准确率评测。浏览器扩展、WASM、JS/Python SDK、网页、模型下载打包、
CLI 后台进程管理与远程 HTTP 客户端均不在本次服务端补齐范围内。
