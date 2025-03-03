package react

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"testing"

	"github.com/sashabaranov/go-openai"

	"easy-k8s/pkg/aiclient"
	ai "easy-k8s/pkg/aimsg"
)

var base_url = "https://dashscope.aliyuncs.com/compatible-mode/v1"

var reactTmpl = `
You are a Kubernetes expert. A user has asked you a question about a Kubernetes issue they are facing. You need to diagnose the problem and provide a solution.

Answer the following questions as best you can. You have access to the following tools:
%s

Use the following format:

Question: the input question you must answer
Thought: you should always think about what to do
Action: the action to take, should be one of %s.
Action Input: the input to the action, use English
Observation: 
	the result of the action from human feedback

... (this Thought/Action/Action Input/Observation can repeat N times)

When you have a response to say to the Human, or if you do not need to use a tool, you MUST use the format:

---
Thought: Do I need to use a tool? No
Final Answer: the final answer to the original input question
---

Begin!

Previous conversation history:
%s

Question: %s`

const systemPrompt = `
您是一名虚拟 k8s（Kubernetes）助手，可以根据用户输入生成 k8s yaml。yaml 保证能被 kubectl apply 命令执行。

#Guidelines
- 不要做任何解释，除了 yaml 内容外，不要输出任何的内容
- 请不要把 yaml 内容，放在 markdown 的 yaml 代码块中
`

type CreateTool struct {
	Name        string
	Description string
	ArgsSchema  string
}

func newCreateTool() *CreateTool {
	return &CreateTool{
		Name:        "CreateTool",
		Description: "用于生成在指定命名空间创建Kubernetes 资源的yaml文件内容，例如创建某 pod 等等",
		ArgsSchema:  `{"type":"object","properties":{"prompt":{"type":"string", "description": "把用户提出的创建资源的prompt原样放在这，不要做任何改变"},"resource":{"type":"string", "description": "指定的 k8s 资源类型，例如 pod, service等等"}}}`}
}

func (c *CreateTool) Run(prompt string) string {
	//让大模型生成yaml
	messages := make([]openai.ChatCompletionMessage, 2)

	messages[0] = openai.ChatCompletionMessage{Role: "system", Content: systemPrompt}
	messages[1] = openai.ChatCompletionMessage{Role: "user", Content: prompt}

	rsp := normalChat(messages)

	// 创建 JSON 对象 {"yaml":"xxx"}
	body := map[string]string{"yaml": rsp.Content}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err.Error()
	}

	return "```json\n" + string(jsonBody) + "\n```"
}

func TestReact(t *testing.T) {
	createTool := newCreateTool()
	createToolDef := "Name: " + createTool.Name + "\nDescription: " + createTool.Description + "\nArgsSchema: " + createTool.ArgsSchema + "\n"
	toolsList := []string{createToolDef}
	toolNames := []string{createTool.Name}
	query := "在default NS下创建pod，名字叫foo-app 标签是app: foo 镜像是higress-registry.cn-hangzhou.cr.aliyuncs.com/higress/http-echo:0.2.4-alpine 参数是\"-text=foo\""
	prompt := fmt.Sprintf(reactTmpl, toolsList, toolNames, "", query)
	ai.MessageStore.AddForUser(prompt)
	first_response := normalChat(ai.MessageStore.ToMessage())
	regexPattern := regexp.MustCompile(`Final Answer:\s*(.*)`)
	finalAnswer := regexPattern.FindStringSubmatch(first_response.Content)
	if len(finalAnswer) > 1 {
		fmt.Println("========最终 GPT 回复========")
		fmt.Println(first_response.Content)
	}
}

func normalChat(message []openai.ChatCompletionMessage) openai.ChatCompletionMessage {
	c := aiclient.NewOpenAiClient(base_url)
	rsp, err := c.CreateChatCompletion(context.TODO(), openai.ChatCompletionRequest{
		Model:    "qwen-max",
		Messages: message,
	})
	if err != nil {
		log.Println(err)
		return openai.ChatCompletionMessage{}
	}

	return rsp.Choices[0].Message
}
