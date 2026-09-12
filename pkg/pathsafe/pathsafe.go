// Package pathsafe 把不可信的相对路径安全地解析到一个基准目录之内。
package pathsafe

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrOutsideBase 表示路径逃出了基准目录。
var ErrOutsideBase = errors.New("path escapes base directory")

// ReadFile opens relative to a pinned directory handle. os.Root enforces
// containment during resolution, even if another process swaps a symlink.
func ReadFile(root *os.Root, rel string) ([]byte, error) {
	if !filepath.IsLocal(rel) || strings.ContainsRune(rel, ':') {
		return nil, ErrOutsideBase
	}
	f, err := root.Open(rel)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("key file must be regular")
	}
	const maxSize = 1 << 20
	b, err := io.ReadAll(io.LimitReader(f, maxSize+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxSize {
		return nil, errors.New("key file too large")
	}
	return b, nil
}

// Within 把 rel 解析成 base 之内的绝对路径。
// Deprecated: this is only a path inspection helper, not a security boundary.
// For untrusted filesystem access use ReadFile with os.OpenRoot instead.
//
// rel 来自请求，必须当成敌意输入：绝对路径直接拒绝，`..` 逃逸直接拒绝，
// 符号链接也要解析之后再判断——否则 base 里放一个指向 /etc 的链接就能绕过检查。
//
// 目标文件不存在时退而检查它的父目录，这样「文件不存在」和「路径越界」
// 才能是两种不同的错误，而不会因为 EvalSymlinks 失败被混为一谈。
func Within(base, rel string) (string, error) {
	if rel == "" {
		return "", errors.New("empty path")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("%w: absolute path not allowed", ErrOutsideBase)
	}
	// Windows 盘符形式（c:foo）在 Linux 上不算绝对路径，这里一并挡掉
	if strings.ContainsRune(rel, ':') {
		return "", fmt.Errorf("%w: volume-qualified path not allowed", ErrOutsideBase)
	}

	cleaned := filepath.Clean(rel)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %s", ErrOutsideBase, rel)
	}

	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	realBase, err := filepath.EvalSymlinks(absBase)
	if err != nil {
		return "", fmt.Errorf("base directory unusable: %w", err)
	}

	joined := filepath.Join(realBase, cleaned)

	// 沿路径往上找到第一个真实存在的祖先来做符号链接解析——目标文件或它的
	// 中间目录都可能还不存在，那属于「文件不存在」，应当由调用方去报，
	// 不该在这里被误判成越界。
	probe := joined
	var rest []string
	for {
		if _, err := os.Lstat(probe); err == nil {
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return "", fmt.Errorf("%w: %s", ErrOutsideBase, rel)
		}
		rest = append([]string{filepath.Base(probe)}, rest...)
		probe = parent
	}

	realProbe, err := filepath.EvalSymlinks(probe)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrOutsideBase, rel)
	}
	// 已存在的那段必须落在 base 之内；剩下不存在的部分是纯名字，拼回去即可。
	if realProbe != realBase && !strings.HasPrefix(realProbe, realBase+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %s", ErrOutsideBase, rel)
	}
	if len(rest) > 0 {
		realProbe = filepath.Join(append([]string{realProbe}, rest...)...)
	}
	return realProbe, nil
}
