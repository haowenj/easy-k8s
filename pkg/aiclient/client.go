package aiclient

import (
	"github.com/sashabaranov/go-openai"
	"os"
)

func NewOpenAiClient(base_url string) *openai.Client {
	token := os.Getenv("DashScope")
	config := openai.DefaultConfig(token)
	config.BaseURL = base_url

	return openai.NewClientWithConfig(config)
}
