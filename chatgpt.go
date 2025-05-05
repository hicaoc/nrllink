package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

var openaiconfig openai.ClientConfig
var openaiclient *openai.Client

func chat(msglist []openai.ChatCompletionMessage) (string, error) {

	resp, err := openaiclient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:    openai.GPT3Dot5Turbo,
			Messages: msglist,
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return "", err
	}

	//fmt.Println(resp.Choices[0].Message.Content)
	return resp.Choices[0].Message.Content, nil
}

func chatgptInit() {
	if conf.OpenAI.Engine == "" {
		openaiconfig = openai.DefaultConfig(conf.OpenAI.APIKEY)

		if conf.OpenAI.BaseURL != "" {
			openaiconfig.BaseURL = conf.OpenAI.BaseURL
		}

		openaiclient = openai.NewClientWithConfig(openaiconfig)
	} else {

		openaiconfig = openai.DefaultAzureConfig(conf.OpenAI.APIKEY, conf.OpenAI.BaseURL)
		openaiconfig.AzureModelMapperFunc = func(model string) string {
			azureModelMapping := map[string]string{
				"gpt-3.5-turbo": conf.OpenAI.Engine,
			}
			return azureModelMapping[model]
		}

		openaiclient = openai.NewClientWithConfig(openaiconfig)

	}
}

var msgMap sync.Map

var help = `你现在是一个培训机构或培训学校的客服，下面是你掌握的信息，后面的问题，你要从下列信息里找到答案回复给我，任何不知道或不确定的问题，你就回答：请你联系学校段老师老师咨询，电话号码是18952025306。
本机构没有app也不需要登录系统，直接在微信公众号菜单里操作。
我问：绑定码在哪里，或者如何绑定等问题，你回答，点击本公众号菜单右下角菜单栏:"我的"，然后再点击:"绑定码"
我问：如何操作相关问题，你要回答，先从本微信公众号菜单获取绑定码，然后将绑定码发给学校老师
我问：剩余多少课时相关问题，你要回答：目前正在开发中，需要等待
我问：如何报名，咨询课程相关问题，你回答：请联系段老师咨询，段老师电话号码是18952025306，这个号码和微信同号，可以直接加微信或者电话咨询。
我问：学校地址，位置在哪里等问题，你回答：我们学校在双龙大道百家湖水晶蓝湾三栋一楼103号，欢迎到学校咨询参观预约试听。
我问：有哪些课程， 你回答： 有架子鼓，钢琴等，欢迎到学校咨询参观免费预约试听
`

func sendMessage(userID string, question string) string {

	var mlist []openai.ChatCompletionMessage

	m1 := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: question,
	}

	if m, ok := msgMap.Load(userID); ok {

		mm := m.([]openai.ChatCompletionMessage)

		if len(mm) > 5 || question == "复位" || question == "结束会话" || question == "重新开始" || question == "reset" {
			msgMap.Delete(userID)

			m1.Content = help
			mlist = append(mlist, m1)
			msgMap.Store(userID, mlist)

			return "会话结束，请重新提问"

		} else {

			mlist = append(mm, m1)

		}
		msgMap.Store(userID, mlist)

	} else {

		m1.Content = help
		mlist = append(mlist, m1)
		m1.Content = question
		mlist = append(mlist, m1)
		msgMap.Store(userID, mlist)
	}

	//fmt.Println("chatgpt mssage list:", mlist)
	m, err := chat(mlist)
	if err != nil {
		return ""
	}

	log.Printf("User %v :%v \n\t\t GPT:%v \n\n", userID, question, m)

	return m

}
