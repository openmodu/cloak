// Command fakellm 是一个假的 OpenAI 兼容端点，用来在不花真 API key 的前提下
// 验证 /api/call 这条链路。
//
// 它做两件事：
//  1. 把收到的 prompt 原样打到标准错误——这就是真正离开本机的内容，
//     你可以直接肉眼确认敏感信息已经变成了占位符；
//  2. 把 prompt 原样当作回复返回，于是 cloak 还原后应当逐字等于原文。
//
// 用法：
//
//	go run ./scripts/fakellm --port 18080
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
)

type chatRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

func main() {
	port := flag.Int("port", 18080, "监听端口")
	flag.Parse()

	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		prompt := ""
		if len(req.Messages) > 0 {
			prompt = req.Messages[len(req.Messages)-1].Content
		}

		log.Printf("\n===== 真正发给「大模型」的内容 =====\n%s\n===== 结束 =====\n", prompt)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": req.Model,
			"choices": []any{
				map[string]any{"message": map[string]string{"role": "assistant", "content": prompt}},
			},
		})
	})

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	log.Printf("假 LLM 已启动: http://%s/v1", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
