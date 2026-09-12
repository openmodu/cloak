package http

import (
	"net/http"

	"github.com/openmodu/cloak/internal/types"
	"github.com/openmodu/cloak/internal/usecase/proxy"
)

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, statusOutput{Status: "ok"})
}

// handleConfig 免重启更新脱敏开关。
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[configRequest](w, r)
	if !ok {
		return
	}
	if req.MaskConfig == nil {
		writeError(w, http.StatusBadRequest, "maskConfig is required")
		return
	}
	s.conf.SetMask(applyMaskConfig(s.conf.Mask(), req.MaskConfig))
	writeOutput(w, statusOutput{Status: "ok"})
}

// handleCall 是完整链路：脱敏 → 调用大模型 → 还原。
func (s *Server) handleCall(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[callRequest](w, r)
	if !ok {
		return
	}
	p := s.proxy
	if req.APIKeyFile != "" {
		if s.proxyFactory == nil {
			writeError(w, http.StatusBadRequest, "request apiKeyFile is not supported")
			return
		}
		var err error
		p, err = s.proxyFactory(req.APIKeyFile)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if p == nil {
		writeError(w, http.StatusServiceUnavailable, "LLM 未配置：启动时请用 --api-key-file 指定 API key 文件")
		return
	}
	temperature := s.temperature
	if req.Temperature != nil {
		temperature = *req.Temperature
	}
	res, err := p.Call(r.Context(), proxy.Request{
		Text:        req.Text,
		Model:       req.Model,
		Temperature: temperature,
	})
	if err != nil {
		s.log.Error("call 失败", "err", err)
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeOutput(w, textOutput{Text: res.Text})
}

func (s *Server) handleMaskText(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[maskRequest](w, r)
	if !ok {
		return
	}
	out, err := s.maskOne(r, req)
	if err != nil {
		s.log.Error("mask_text 失败", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOutput(w, out)
}

func (s *Server) handleMaskTextBatch(w http.ResponseWriter, r *http.Request) {
	reqs, ok := decode[[]maskRequest](w, r)
	if !ok {
		return
	}
	outs := make([]maskOutput, 0, len(reqs))
	for _, req := range reqs {
		out, err := s.maskOne(r, req)
		if err != nil {
			s.log.Error("mask_text_batch 失败", "err", err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		outs = append(outs, out)
	}
	writeOutput(w, outs)
}

func (s *Server) handleRestoreText(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[restoreRequest](w, r)
	if !ok {
		return
	}
	out, err := s.restoreOne(r, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOutput(w, out)
}

func (s *Server) handleRestoreTextBatch(w http.ResponseWriter, r *http.Request) {
	reqs, ok := decode[[]restoreRequest](w, r)
	if !ok {
		return
	}
	outs := make([]textOutput, 0, len(reqs))
	for _, req := range reqs {
		out, err := s.restoreOne(r, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		outs = append(outs, out)
	}
	writeOutput(w, outs)
}

// maskOne 脱敏一条文本并把还原凭据编码成不透明字符串交还调用方。
// 服务端不留存凭据，还原时由调用方带回来。
func (s *Server) maskOne(r *http.Request, req maskRequest) (maskOutput, error) {
	masked, meta, err := s.masker.MaskWithLanguage(r.Context(), req.Text, types.Language(req.Language))
	if err != nil {
		return maskOutput{}, err
	}
	encoded, err := meta.EncodeAIFW()
	if err != nil {
		return maskOutput{}, err
	}
	return maskOutput{Text: masked, MaskMeta: encoded}, nil
}

func (s *Server) restoreOne(r *http.Request, req restoreRequest) (textOutput, error) {
	meta, err := types.DecodeMaskMeta(req.MaskMeta)
	if err != nil {
		return textOutput{}, err
	}
	restored, err := s.restorer.Restore(r.Context(), req.Text, meta)
	if err != nil {
		return textOutput{}, err
	}
	return textOutput{Text: restored}, nil
}
