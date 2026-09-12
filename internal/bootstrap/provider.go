package bootstrap

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/google/wire"

	"github.com/openmodu/cloak/internal/repo/confrepo"
	"github.com/openmodu/cloak/internal/repo/langrepo"
	"github.com/openmodu/cloak/internal/repo/llmrepo"
	"github.com/openmodu/cloak/internal/repo/nerrepo"
	"github.com/openmodu/cloak/internal/repo/regexrepo"
	httptransport "github.com/openmodu/cloak/internal/transport/http"
	"github.com/openmodu/cloak/internal/types"
	"github.com/openmodu/cloak/internal/usecase"
	"github.com/openmodu/cloak/internal/usecase/masker"
	"github.com/openmodu/cloak/internal/usecase/proxy"
	"github.com/openmodu/cloak/internal/usecase/restorer"
	"github.com/openmodu/cloak/pkg/onnxrt"
	"github.com/openmodu/cloak/pkg/pathsafe"
)

// RepoSet 汇集 repo 层实现，并把它们绑定到 usecase 声明的接口上。
// 换实现（正则 → NER、启发式语言判定 → 第三方库）只需改这里的绑定。
var RepoSet = wire.NewSet(
	regexrepo.NewDefaultSet,
	ProvideNERRecognizers,
	ProvideRecognizers,

	langrepo.New,
	wire.Bind(new(usecase.LangDetector), new(*langrepo.Detector)),

	confrepo.NewDefault,
	wire.Bind(new(usecase.ConfigStore), new(*confrepo.Memory)),
)

// UseCaseSet 汇集用例对象的构造。
var UseCaseSet = wire.NewSet(
	ProvideMasker,
	restorer.New,
)

// ProviderSet 是完整的装配集合。
var ProviderSet = wire.NewSet(
	RepoSet,
	UseCaseSet,
	NewApp,
)

// ProvideRecognizers 决定启用哪些识别器以及它们的顺序。
//
// 正则识别器按实体类型枚举顺序排在前，NER 的结果接在它们后面；
// 这个顺序影响完全同分同范围区间的取舍，不要随意调整。
// 要再加一种识别方式，在这里追加一个实现即可，masker 不需要任何改动。
func ProvideRecognizers(rx []*regexrepo.Recognizer, ner []*nerrepo.Recognizer) []usecase.Recognizer {
	out := make([]usecase.Recognizer, 0, len(rx)+len(ner))
	for _, r := range rx {
		out = append(out, r)
	}
	for _, r := range ner {
		out = append(out, r)
	}
	return out
}

// 中英文各挂一个模型，按语言分流。模型 id 同时也是它在模型根目录下的相对路径。
// 默认模型。标签集须是 PER / ORG / LOC 这一套，否则 labelToEntityType 认不出来。
const (
	DefaultENModelID = "funstory-ai/neurobert-mini"
	DefaultZHModelID = "ckiplab/bert-tiny-chinese-ner"
)

// nerModelsFor 决定启用哪些模型、各自负责哪种语言。
func nerModelsFor(cfg Config) []struct {
	id      string
	forLang func(types.Language) bool
} {
	en, zh := cfg.ENModelID, cfg.ZHModelID
	if en == "" {
		en = DefaultENModelID
	}
	if zh == "" {
		zh = DefaultZHModelID
	}
	return []struct {
		id      string
		forLang func(types.Language) bool
	}{
		{en, func(l types.Language) bool { return !l.IsChinese() }},
		{zh, func(l types.Language) bool { return l.IsChinese() }},
	}
}

// ProvideNERRecognizers 按约定的目录布局加载模型：
// <ModelsDir>/<model-id>/onnx/model_quantized.onnx 及同目录的 config.json、vocab。
//
// 任何一步不成立都只打一条警告并退化成纯正则，而不是让整个服务起不来：
// 少一类识别能力，远好过整个脱敏网关不可用。
func ProvideNERRecognizers(cfg Config) ([]*nerrepo.Recognizer, error) {
	if cfg.ModelsDir == "" {
		return nil, nil
	}
	var out []*nerrepo.Recognizer
	for _, m := range nerModelsFor(cfg) {
		dir := filepath.Join(cfg.ModelsDir, filepath.FromSlash(m.id))
		modelPath := filepath.Join(dir, "onnx", "model_quantized.onnx")
		if _, err := os.Stat(modelPath); err != nil {
			slog.Warn("NER 模型缺失，该语言退化为纯正则", "model", m.id, "path", modelPath)
			continue
		}
		sess, err := onnxrt.Open(modelPath)
		if err != nil {
			slog.Warn("NER 模型加载失败，该语言退化为纯正则", "model", m.id, "err", err)
			continue
		}
		opts := []nerrepo.Option{nerrepo.WithLanguageFilter(m.forLang)}
		if m.forLang(types.LangZhHans) {
			converter, err := nerrepo.NewOpenCC()
			if err != nil {
				slog.Warn("OpenCC 不可用，简繁转换已禁用", "err", err)
			} else {
				opts = append(opts, nerrepo.WithConverter(converter))
			}
		}
		rec, err := nerrepo.LoadFromDir("ner:"+m.id, dir, sess, opts...)
		if err != nil {
			slog.Warn("NER 模型初始化失败，该语言退化为纯正则", "model", m.id, "err", err)
			_ = sess.Close()
			continue
		}
		slog.Info("已加载 NER 模型", "model", m.id)
		out = append(out, rec)
	}
	return out, nil
}

// ProvideMasker 把注入的依赖翻译成 masker 的函数选项。
func ProvideMasker(rs []usecase.Recognizer, d usecase.LangDetector, c usecase.ConfigStore, cfg Config) *masker.Masker {
	c.SetMask(types.ApplyMaskConfig(c.Mask(), cfg.MaskConfig))
	return masker.New(
		masker.WithRecognizers(rs...),
		masker.WithLangDetector(d),
		masker.WithConfigStore(c),
		masker.WithAddressFallback(cfg.AddressFallback),
	)
}

// ServerSet 在基础装配之上补齐 HTTP 服务需要的部件。
var ServerSet = wire.NewSet(
	ProviderSet,
	ProvideLLMClient,
	ProvideProxy,
	ProvideHTTPServer,
)

// ProvideLLMClient 只在配置了 API key 文件时才构造客户端。
// 没配就返回 nil，让上层明确地把「未接入大模型」这件事告诉调用方。
func ProvideLLMClient(cfg Config) (usecase.LLMClient, error) {
	if cfg.APIKeyFile == "" {
		return nil, nil
	}
	return llmrepo.NewFromFile(cfg.APIKeyFile)
}

// ProvideProxy 在没有 LLM 客户端时返回 nil，由 HTTP 层据此回 503。
func ProvideProxy(m *masker.Masker, r *restorer.Restorer, llm usecase.LLMClient) *proxy.Proxy {
	if llm == nil {
		return nil
	}
	return proxy.New(m, r, llm)
}

func ProvideHTTPServer(app *App, p *proxy.Proxy, cfg Config) *httptransport.Server {
	return httptransport.New(app.Masker, app.Restorer, app.Conf,
		httptransport.WithProxy(p),
		httptransport.WithAPIKey(cfg.HTTPAPIKey),
		httptransport.WithTemperature(cfg.Temperature),
		httptransport.WithProxyFactory(requestAPIKeyFactory(app, cfg.APIKeyDir)),
	)
}

// requestAPIKeyFactory 构造「请求级 apiKeyFile」的处理函数。
//
// 没配置 APIKeyDir 就返回 nil，HTTP 层据此拒掉这类请求——默认关闭，
// 需要显式打开。打开之后请求里的路径只能是相对 APIKeyDir 的相对路径，
// 绝对路径、`..` 逃逸、指向目录外的符号链接一律拒绝。
func requestAPIKeyFactory(app *App, dir string) func(string) (*proxy.Proxy, error) {
	if dir == "" {
		return nil
	}
	root, openErr := os.OpenRoot(dir)
	return func(path string) (*proxy.Proxy, error) {
		if openErr != nil {
			return nil, openErr
		}
		data, err := pathsafe.ReadFile(root, path)
		if err != nil {
			return nil, err
		}
		key, err := llmrepo.ParseAPIKeyFile(data)
		if err != nil {
			return nil, err
		}
		return proxy.New(app.Masker, app.Restorer, llmrepo.New(key)), nil
	}
}
