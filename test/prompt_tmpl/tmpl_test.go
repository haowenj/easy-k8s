package prompt_tmpl

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"easy-k8s/pkg/aiclient"
	ai "easy-k8s/pkg/aimsg"

	"github.com/sashabaranov/go-openai"
)

var base_url = "https://dashscope.aliyuncs.com/compatible-mode/v1"

const Template = `Answer the following questions as best you can. You have access to the following tools:


%s


Use the following format:


Question: the input question you must answer
Thought: you should always think about what to do
Action: the action to take, should be one of [%s]
Action Input: the input to the action
Observation: the result of the action
Thought: I now know the final answer
Final Answer: the final answer to the original input question


Begin!


Question: %s
`

const AddToolName = `AddTool`
const SubToolName = `SubTool`
const AddToolDescription = `
Use this tool for addition calculations.
	example:
		1+2 =?
	then Action Input is: 1,2
`
const SubToolDescription = `
Use this tool for subtraction calculations.
	example:
		1-2 =?
	then Action Input is: 1,2
`
const AddToolParam = `{"type":"object","properties":{"numbers":{"type":"array","items":{"type":"integer"}}}}`
const SubToolParam = `{"type":"object","properties":{"numbers":{"type":"array","items":{"type":"integer"}}}}`

func TestPrompt(t *testing.T) {
	addtool := AddToolName + ":" + AddToolDescription + "\nparam: \n" + AddToolParam
	subtool := SubToolName + ":" + SubToolDescription + "\nparam: \n" + SubToolParam
	toolsL := []string{addtool, subtool}
	tool_names := []string{AddToolName, SubToolName}
	query := "156+223+347+489-599-65=? Just give me a number result"

	prompt := fmt.Sprintf(Template, toolsL, tool_names, query)

	ai.MessageStore.AddForUser(prompt)
	i := 1
	for {
		first_response := NormalChat(ai.MessageStore.ToMessage())
		fmt.Printf("========第%d轮回答========\n", i)
		//fmt.Println(first_response.Content)
		regexPattern := regexp.MustCompile(`Final Answer:\s*(.*)`)
		finalAnswer := regexPattern.FindStringSubmatch(first_response.Content)
		if len(finalAnswer) > 1 {
			fmt.Println("========最终 GPT 回复========")
			fmt.Println(first_response.Content)
			break
		}

		ai.MessageStore.AddForAssistant(first_response)
		regexAction := regexp.MustCompile(`Action:\s*(.*?)[.\n]`)
		regexActionInput := regexp.MustCompile(`Action Input:\s*(.*?)[.\n]`)
		action := regexAction.FindStringSubmatch(first_response.Content)
		actionInput := regexActionInput.FindStringSubmatch(first_response.Content)
		if len(action) > 1 && len(actionInput) > 1 {
			i++
			result := 0
			//需要调用工具
			if action[1] == "AddTool" {
				fmt.Println("calls AddTool")
				result = AddTool(actionInput[1])
			} else if action[1] == "SubTool" {
				fmt.Println("calls SubTool")
				result = SubTool(actionInput[1])
			}
			fmt.Println("========函数返回结果========")
			fmt.Println(result)

			Observation := "Observation: " + strconv.Itoa(result)
			prompt = first_response.Content + Observation
			fmt.Printf("========第%d轮的prompt========\n", i)
			fmt.Println(prompt)
			ai.MessageStore.AddForUser(prompt)
		}
	}
}

func AddTool(numbers string) int {
	num := strings.Split(numbers, ",")
	inum0, _ := strconv.Atoi(num[0])
	inum1, _ := strconv.Atoi(num[1])
	return inum0 + inum1
}

func SubTool(numbers string) int {
	num := strings.Split(numbers, ",")
	inum0, _ := strconv.Atoi(num[0])
	inum1, _ := strconv.Atoi(num[1])
	return inum0 - inum1
}

func NormalChat(message []openai.ChatCompletionMessage) openai.ChatCompletionMessage {
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
