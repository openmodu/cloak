# aifw 服务端兼容情况

比较基线：aifw `74ee556`。本页区分实现覆盖与真实模型验证。

| 项目 | cloak 行为 |
|---|---|
| 脱敏配置 | YAML `mask_config` 启动时应用，`maskAll` 先执行、单项覆盖；HTTP 可热更新 |
| 默认温度 | `--temperature` > `CLOAK_TEMPERATURE` > YAML；请求显式传 0 会覆盖默认值 |
| 服务日志 | `--log-level` > `CLOAK_LOG_LEVEL` > YAML |
| 请求级 apiKeyFile | 使用服务端本地文件构造本次调用的客户端，不覆盖其他请求 |
| 错误响应 | `{message, code}` 对象；保留 cloak 的 HTTP 4xx/5xx，不照搬上游业务异常返回 200 |
| 自动语言 | 省略、空字符串、`auto` 均触发自动检测；检测算法仍是 cloak 的启发式算法 |
| 凭据 | HTTP/CLI JSON 输出小端 aifw ABI；读取同时兼容旧 cloak JSON |
| Unicode 正则 | 内置规则的数字、空白及词边界使用 Unicode 集合；自定义规则中的复杂嵌套/否定类不保证 Rust 全语法兼容 |
| NER 分词 | 共享转换器串行保护，已有并发回归；真实模型尚需验证 |
| NER 序列长度 | 取模型 `config.json` 的 `max_position_embeddings`，读不到退回 512 |
| 地址融合兜底 | 融合产不出结果时保留 NER 原始地址区间；仅当该段本身够不到隐私阈值才丢弃 |
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

真实模型检查需要提供两个模型目录及 ONNX Runtime 动态库：

```bash
CLOAK_TEST_MODELS_DIR=/path/to/models \
CLOAK_ONNXRUNTIME_LIB=/path/to/libonnxruntime.so make test-onnx
```

默认模型是 `Xenova/bert-base-NER`（英文）与
`Xenova/bert-base-multilingual-cased-ner-hrl`（中文），可用 `--en-model` /
`--zh-model`、`CLOAK_EN_MODEL_ID` / `CLOAK_ZH_MODEL_ID` 或 YAML 覆盖。
标签集须是 PER / ORG / LOC 这一套。

已跑通的真实模型验证（ONNX Runtime 1.30.0，CPU）：

| 样本 | 结果 |
|---|---|
| `John Smith works at Microsoft in New York.` | 人名 / 机构 / 地名三项全中，分数 >0.99 |
| `王小明住在北京市朝阳区建国路88号。` | 人名与地址均命中，偏移落在字符边界上 |
| `公司在苏州工业园区星海街星海广场2栋18层1802室办公。` | NER 只给出「苏州」与「星海街星海广场」两段碎片，地址融合补全为完整地址 |
| `请把合同寄到深圳市南山区科技南十二路8-2号科兴科学园C座5层。` | 融合产不出结果（门牌号被层级倒挂清理删掉），回退到 NER 边界脱敏 |
| `我在上海市浦东新区上班，离家很近。` | 只到区县，够不到隐私阈值，按设计不脱敏 |

此命令不能替代同模型、同语料的差分测试。中文地址融合的逐行对拍在当前代码上仍然成立
（`pkg/zhaddr` 自对拍通过后未再改动）。浏览器扩展、WASM、JS/Python SDK、网页、模型下载打包、
CLI 后台进程管理与远程 HTTP 客户端均不在本次服务端补齐范围内。
