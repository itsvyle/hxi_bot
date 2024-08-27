package main

import (
	"context"

	openai "github.com/sashabaranov/go-openai"
)

type OpenAIChatBot struct {
	openAIKey    string
	modelName    string
	openAIClient *openai.Client
	maxTokens    int
	temperature  float32
}

func CreateNewOpenAIChatBot(openAIKey string, modelName string, maxTokens int, temperature float64) *OpenAIChatBot {
	openAIClient := openai.NewClient(openAIKey)
	return &OpenAIChatBot{
		openAIClient: openAIClient,
		openAIKey:    openAIKey,
		modelName:    modelName,
		maxTokens:    maxTokens,
		temperature:  float32(temperature),
	}
}

func (chatBot *OpenAIChatBot) Query(messages *[]openai.ChatCompletionMessage) (string, error) {
	resp, err := chatBot.openAIClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       chatBot.modelName,
			Messages:    *messages,
			MaxTokens:   chatBot.maxTokens,
			Temperature: chatBot.temperature,
		},
	)
	if err != nil {
		return "", err
	}
	res := resp.Choices[0].Message.Content

	return res, nil
}
