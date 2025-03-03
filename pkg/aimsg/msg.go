package aimsg

import "github.com/sashabaranov/go-openai"

// 枚举出角色
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
	RoleTool      = "tool"
)

type ChatMessages []*ChatMessage

type ChatMessage struct {
	Msg openai.ChatCompletionMessage
}

var MessageStore ChatMessages

func init() {
	MessageStore = make(ChatMessages, 0)

}

// Clear 定义人设
func (cm *ChatMessages) Clear(msg string) {
	*cm = make([]*ChatMessage, 0) //重新初始化
	cm.AddForSystem(msg)
}

// AddFor 添加角色和对应的prompt
func (cm *ChatMessages) AddFor(msg string, role string) {
	*cm = append(*cm, &ChatMessage{
		Msg: openai.ChatCompletionMessage{
			Role:    role,
			Content: msg,
		},
	})
}

// AddForToolCall 添加角色和对应的prompt
func (cm *ChatMessages) AddForToolCall(rsp openai.ChatCompletionMessage, role string) {
	*cm = append(*cm, &ChatMessage{
		Msg: openai.ChatCompletionMessage{
			Role:         role,
			Content:      rsp.Content,
			FunctionCall: rsp.FunctionCall,
			ToolCalls:    rsp.ToolCalls,
		},
	})
}

// AddForAssistant 添加Assistant角色的prompt
func (cm *ChatMessages) AddForAssistant(rsp openai.ChatCompletionMessage) {
	cm.AddForToolCall(rsp, RoleAssistant)

}

// AddForSystem 添加System角色的prompt
func (cm *ChatMessages) AddForSystem(msg string) {
	cm.AddFor(msg, RoleSystem)
}

// AddForUser 添加User角色的prompt
func (cm *ChatMessages) AddForUser(msg string) {
	cm.AddFor(msg, RoleUser)
}

// AddForTool 添加Tool角色的prompt
func (cm *ChatMessages) AddForTool(msg string, name string, toolCallID string) {
	*cm = append(*cm, &ChatMessage{
		Msg: openai.ChatCompletionMessage{
			Role:       RoleTool,
			Content:    msg,
			Name:       name,
			ToolCallID: toolCallID,
		},
	})
}

// ToMessage 组装prompt
func (cm *ChatMessages) ToMessage() []openai.ChatCompletionMessage {
	ret := make([]openai.ChatCompletionMessage, len(*cm))
	for index, c := range *cm {
		ret[index] = c.Msg
	}
	return ret
}

// GetLast 得到返回的消息
func (cm *ChatMessages) GetLast() string {
	if len(*cm) == 0 {
		return "什么都没找到"
	}

	return (*cm)[len(*cm)-1].Msg.Content
}
