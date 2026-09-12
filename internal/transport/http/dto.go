package http

// 请求与响应结构。响应统一是 {"output": ..., "error": ...} 信封，
// 成功时 error 为 null，失败时 output 为 null。

type envelope struct {
	Output any       `json:"output"`
	Error  *apiError `json:"error"`
}

type apiError struct {
	Message string `json:"message"`
	Code    any    `json:"code"`
}

type statusOutput struct {
	Status string `json:"status"`
}

type textOutput struct {
	Text string `json:"text"`
}

type maskOutput struct {
	Text     string `json:"text"`
	MaskMeta string `json:"maskMeta"`
}

type callRequest struct {
	Text        string   `json:"text"`
	Model       string   `json:"model"`
	Temperature *float32 `json:"temperature"`
	APIKeyFile  string   `json:"apiKeyFile"`
}

type maskRequest struct {
	Text     string `json:"text"`
	Language string `json:"language"`
}

type restoreRequest struct {
	Text     string `json:"text"`
	MaskMeta string `json:"maskMeta"`
}

type configRequest struct {
	MaskConfig map[string]*bool `json:"maskConfig"`
}
