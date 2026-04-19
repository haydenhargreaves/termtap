package proxy

import (
	"bytes"
	"strings"
)

const maxPreviewBytes = 1024 * 64 // 64 kb (maybe we want 256kb)

type bodyPreview struct {
	enabled   bool
	truncated bool
	buf       bytes.Buffer
}

func newBodyPreview(contentType string) *bodyPreview {
	return &bodyPreview{enabled: canDisplayContent(contentType)}
}

func (p *bodyPreview) Write(data []byte) {
	if p == nil || !p.enabled || len(data) == 0 {
		return
	}

	remaining := maxPreviewBytes - p.buf.Len()
	if remaining <= 0 {
		p.truncated = true
		return
	}

	if len(data) > remaining {
		data = data[:remaining]
		p.truncated = true
	}

	_, _ = p.buf.Write(data)
}

func (p *bodyPreview) Preview() []byte {
	if p == nil || !p.enabled || p.buf.Len() == 0 {
		return []byte{}
	}

	text := strings.ReplaceAll(p.buf.String(), "\n", "\\n")
	if p.truncated {
		text += "..."
	}

	return []byte(text)
}
