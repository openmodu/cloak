//go:build !cloak_onnx

// Package onnxrt 在未带 cloak_onnx 编译标签时只提供占位实现，
// 让默认构建保持纯 Go、零系统依赖。
package onnxrt

import (
	"context"
	"errors"
)

// ErrNotBuilt 表示当前二进制没有把 ONNX Runtime 编进来。
var ErrNotBuilt = errors.New("onnxrt: 未启用 ONNX 支持，请用 -tags cloak_onnx 重新编译")

type Session struct{}

func Init(string) error { return ErrNotBuilt }

func Open(string) (*Session, error) { return nil, ErrNotBuilt }

func (s *Session) Infer(context.Context, []int64, []int64, []int64) ([][]float32, error) {
	return nil, ErrNotBuilt
}

func (s *Session) Close() error { return nil }
