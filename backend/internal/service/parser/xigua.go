package parser

import (
	"strings"
)

// XiguaParser 西瓜视频解析器
type XiguaParser struct{}

func (p *XiguaParser) Platform() string {
	return "xigua"
}

func (p *XiguaParser) Match(url string) bool {
	return strings.Contains(url, "ixigua.com")
}

func (p *XiguaParser) Parse(url string) (*ParseResult, error) {
	return &ParseResult{
		Success: false,
		Error:   "西瓜视频平台暂不支持",
	}, nil
}
