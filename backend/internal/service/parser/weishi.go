package parser

import (
	"strings"
)

// WeishiParser 微视解析器
type WeishiParser struct{}

func (p *WeishiParser) Platform() string {
	return "weishi"
}

func (p *WeishiParser) Match(url string) bool {
	return strings.Contains(url, "weishi.qq.com")
}

func (p *WeishiParser) Parse(url string) (*ParseResult, error) {
	return &ParseResult{
		Success: false,
		Error:   "微视平台暂不支持",
	}, nil
}
