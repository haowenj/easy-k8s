package test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/sashabaranov/go-openai"
)

var base_url = "https://dashscope.aliyuncs.com/compatible-mode/v1"

var calculate = openai.Tool{
	Type: "function",
	Function: &openai.FunctionDefinition{
		Name:        "AddTool",
		Description: "使用此工具进行加法计算，例如用户输入1+2等于几，则这个工具输出1,2，如果用户输入1+2-1等于几，则这个工具输出1,2,-1",
		Parameters:  `{"type":"object","properties":{"numbers":{"type":"array","items":{"type":"integer"}}}}`,
	},
}

func TestAgent(t *testing.T) {
	cm := &MessageStore
	cm.AddFor("user", "请用毛主席的语气鼓励我一下，眼下的苦难都是暂时的，我们要坚持下去，要对未来充满信心。")
	rsp := ToolChat(cm.ToMessage(), []openai.Tool{calculate})
	//说明调用了工具
	if len(rsp.ToolCalls) > 0 {
		var args struct {
			Numbers []int `json:"numbers"`
		}
		err := json.Unmarshal([]byte(rsp.ToolCalls[0].Function.Arguments), &args)
		if err != nil {
			fmt.Println("err", err)
			return
		}
		sum := 0
		for _, v := range args.Numbers {
			sum += v
		}
		cm.AddForAssistant("assistant", rsp.Content, rsp.ToolCalls)
		cm.AddForTool("tool", fmt.Sprintf("%d", sum), rsp.ToolCalls[0].Function.Name, rsp.ToolCalls[0].ID)
		response := ToolChat(cm.ToMessage(), []openai.Tool{calculate})
		fmt.Println(response.Content)
	} else {
		fmt.Println(rsp.Content)
	}
}

var MessageStore ChatMessages

type ChatMessages []openai.ChatCompletionMessage

func (cm *ChatMessages) AddForTool(role string, msg string, name string, toolCallID string) {
	*cm = append(*cm, openai.ChatCompletionMessage{
		Role:       role,
		Content:    msg,
		Name:       name,
		ToolCallID: toolCallID,
	})
}
func (cm *ChatMessages) AddForAssistant(role string, msg string, toolCalls []openai.ToolCall) {
	*cm = append(*cm, openai.ChatCompletionMessage{
		Role:      role,
		Content:   msg,
		ToolCalls: toolCalls,
	})
}
func (cm *ChatMessages) AddFor(role string, msg string) {
	*cm = append(*cm, openai.ChatCompletionMessage{
		Role:    role,
		Content: msg,
	})
}

func (cm *ChatMessages) ToMessage() []openai.ChatCompletionMessage {
	ret := make([]openai.ChatCompletionMessage, len(*cm))
	for index, c := range *cm {
		ret[index] = c
	}
	return ret
}
func NewOpenAiClient() *openai.Client {
	token := os.Getenv("DashScope")
	config := openai.DefaultConfig(token)
	config.BaseURL = base_url

	return openai.NewClientWithConfig(config)
}
func ToolChat(message []openai.ChatCompletionMessage, tools []openai.Tool) openai.ChatCompletionMessage {
	c := NewOpenAiClient()
	rsp, err := c.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model:      "qwen-max",
		Messages:   message,
		Tools:      tools,
		ToolChoice: "auto",
	})
	if err != nil {
		fmt.Println(err)
		return openai.ChatCompletionMessage{}
	}
	return rsp.Choices[0].Message
}
