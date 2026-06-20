package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

// MiniMaxClient MiniMax API客户端
type MiniMaxClient struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// Message 消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 请求
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// ChatResponse 响应
type ChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func NewMiniMaxClient(apiKey, model string) *MiniMaxClient {
	return &MiniMaxClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://api.minimax.chat/v1/text/chatcompletion_v2",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Chat 对话
func (c *MiniMaxClient) Chat(messages []Message) (string, error) {
	if c.apiKey == "" {
		return "", errors.New("MiniMax API Key未配置")
	}

	reqBody := ChatRequest{
		Model:    c.model,
		Messages: messages,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", errors.New("AI未返回响应")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// ChatWithPreset 使用预设对话
func (c *MiniMaxClient) ChatWithPreset(presetName, presetContent, userMessage string) (string, error) {
	return c.Chat([]Message{
		{Role: "system", Content: presetContent},
		{Role: "user", Content: userMessage},
	})
}
