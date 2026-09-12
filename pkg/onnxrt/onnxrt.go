//go:build cloak_onnx

// Package onnxrt 用 ONNX Runtime 跑本地模型推理。
//
// 这一层要 cgo 并在运行时 dlopen libonnxruntime，因此挂在 cloak_onnx 编译标签后面：
// 默认构建是纯 Go、零系统依赖的；需要本地 NER 时按 README 的说明带上标签编译。
//
//	go build -tags cloak_onnx ./cmd/cloakd
package onnxrt

import (
	"context"
	"fmt"
	"os"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

var initOnce struct {
	sync.Once
	err error
}

// Init 初始化 ONNX Runtime 环境，进程内只做一次。
// libPath 为空时按 CLOAK_ONNXRUNTIME_LIB 环境变量取值，再空则用运行时的默认查找逻辑。
func Init(libPath string) error {
	initOnce.Do(func() {
		if libPath == "" {
			libPath = os.Getenv("CLOAK_ONNXRUNTIME_LIB")
		}
		if libPath != "" {
			ort.SetSharedLibraryPath(libPath)
		}
		initOnce.err = ort.InitializeEnvironment()
	})
	return initOnce.err
}

// Session 是一次 token 分类推理会话，实现 nerrepo.Inferencer。
type Session struct {
	mu        sync.Mutex
	sess      *ort.DynamicAdvancedSession
	inputs    []string
	output    string
	numLabels int
}

// Open 打开模型文件。输入名按模型自报的顺序绑定，兼容只有 input_ids/attention_mask
// 而没有 token_type_ids 的模型。
func Open(modelPath string) (*Session, error) {
	if err := Init(""); err != nil {
		return nil, fmt.Errorf("onnxrt: 初始化失败: %w", err)
	}
	inputInfo, outputInfo, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		return nil, fmt.Errorf("onnxrt: 读取模型输入输出失败: %w", err)
	}
	if len(outputInfo) == 0 {
		return nil, fmt.Errorf("onnxrt: 模型没有输出")
	}

	inputNames := make([]string, 0, len(inputInfo))
	for _, in := range inputInfo {
		inputNames = append(inputNames, in.Name)
	}
	outputName := outputInfo[0].Name

	// 输出形状通常是 [batch, seq, num_labels]，最后一维是标签数
	numLabels := 0
	if d := outputInfo[0].Dimensions; len(d) > 0 && d[len(d)-1] > 0 {
		numLabels = int(d[len(d)-1])
	}

	sess, err := ort.NewDynamicAdvancedSession(modelPath, inputNames, []string{outputName}, nil)
	if err != nil {
		return nil, fmt.Errorf("onnxrt: 创建会话失败: %w", err)
	}
	return &Session{sess: sess, inputs: inputNames, output: outputName, numLabels: numLabels}, nil
}

// Infer 执行一次推理，返回 [seqLen][numLabels] 的 logits。
func (s *Session) Infer(_ context.Context, inputIDs, attentionMask, tokenTypeIDs []int64) ([][]float32, error) {
	seqLen := len(inputIDs)
	if seqLen == 0 {
		return nil, nil
	}
	shape := ort.NewShape(1, int64(seqLen))

	byName := map[string][]int64{
		"input_ids":      inputIDs,
		"attention_mask": attentionMask,
		"token_type_ids": tokenTypeIDs,
	}

	values := make([]ort.Value, 0, len(s.inputs))
	defer func() {
		for _, v := range values {
			_ = v.Destroy()
		}
	}()
	for _, name := range s.inputs {
		data, ok := byName[name]
		if !ok {
			// 模型要一个我们不认识的输入，按全零补上，避免直接失败
			data = make([]int64, seqLen)
		}
		t, err := ort.NewTensor(shape, data)
		if err != nil {
			return nil, fmt.Errorf("onnxrt: 构造输入 %s 失败: %w", name, err)
		}
		values = append(values, t)
	}

	out, err := ort.NewEmptyTensor[float32](ort.NewShape(1, int64(seqLen), int64(s.numLabels)))
	if err != nil {
		return nil, fmt.Errorf("onnxrt: 构造输出张量失败: %w", err)
	}
	defer out.Destroy()

	s.mu.Lock()
	err = s.sess.Run(values, []ort.Value{out})
	s.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("onnxrt: 推理失败: %w", err)
	}

	flat := out.GetData()
	logits := make([][]float32, seqLen)
	for i := 0; i < seqLen; i++ {
		row := make([]float32, s.numLabels)
		copy(row, flat[i*s.numLabels:(i+1)*s.numLabels])
		logits[i] = row
	}
	return logits, nil
}

func (s *Session) Close() error {
	if s.sess == nil {
		return nil
	}
	return s.sess.Destroy()
}
