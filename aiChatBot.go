package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/itsvyle/hxi_bot/config"

	openai "github.com/sashabaranov/go-openai"
)

type ServiceAiChatBot struct {
	config       *config.ConfigSchemaJsonAiChatServicesElem
	openAIClient *openai.Client
	botTrigger   string
}

func CreateNewServiceAiChatBot(config *config.ConfigSchemaJsonAiChatServicesElem) *ServiceAiChatBot {
	return &ServiceAiChatBot{
		config: config,
	}
}

func (s *ServiceAiChatBot) InitAiChatBot(discordSession *discordgo.Session) {
	s.openAIClient = openai.NewClient(s.config.OpenAIAPiKey)

	s.botTrigger = fmt.Sprintf("<@%s>", discordSession.State.User.ID)

	discordSession.AddHandler(func(session *discordgo.Session, message *discordgo.MessageCreate) {
		if message.Author.ID == session.State.User.ID {
			return
		}
		if !strings.Contains(message.Content, s.botTrigger) {
			mentionsMe := false
			for _, user := range message.Mentions {
				if user.ID == session.State.User.ID {
					mentionsMe = true
					break
				}
			}
			if !mentionsMe {
				return
			}
		}
		slog.Info("------------------------------------------------")

		_ = session.ChannelTyping(message.ChannelID)

		chain := s.getMessageChain(session, message.Message)
		s1, _ := json.Marshal(chain)
		println(string(s1))

		chain = append([]openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: s.config.Prompt,
			},
		}, chain...)

		resp, err := s.openAIClient.CreateChatCompletion(
			context.Background(),
			openai.ChatCompletionRequest{
				Model:       s.config.OpenAIModelName,
				Messages:    chain,
				MaxTokens:   s.config.MaxTokens,
				Temperature: float32(s.config.Temperature),
			},
		)
		if err != nil {
			_, _ = session.ChannelMessageSendReply(message.ChannelID, "Error getting response from AI", message.Reference())
			slog.With("error", err).Error("Error getting response from AI")
			return
		}

		res := resp.Choices[0].Message.Content

		slog.Info("AI response", "response", res)

		_, err = session.ChannelMessageSendReply(message.ChannelID, res, message.Reference())
		if err != nil {
			slog.With("error", err, "channelID", message.ChannelID).Error("Error sending message to channel")
			return
		}
	})

	slog.Info("Initialized AI chat bot", "botName", s.config.BotName, "prompt", s.config.Prompt)
}

func (s *ServiceAiChatBot) processMessageContentInput(message string) string {
	return strings.Replace(message, s.botTrigger, "@"+s.config.BotName, 1)
}

func (s *ServiceAiChatBot) getMessageChain(discordSession *discordgo.Session, message *discordgo.Message) []openai.ChatCompletionMessage {
	r := make([]openai.ChatCompletionMessage, 1, s.config.MaxContextSize)
	r[0] = openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: s.processMessageContentInput(message.Content),
	}
	i := 1
	var err error
	for message.MessageReference != nil && i < s.config.MaxContextSize {
		message, err = discordSession.ChannelMessage(message.ChannelID, message.MessageReference.MessageID)
		if err != nil {
			slog.With("error", err).Error("Error getting message reference")
			return reverseArray(r)
		}

		role := openai.ChatMessageRoleUser
		if message.Author.Bot {
			role = openai.ChatMessageRoleAssistant
		}
		r = append(r, openai.ChatCompletionMessage{
			Role:    role,
			Content: s.processMessageContentInput(message.Content),
		})
		i++
	}
	return reverseArray(r)
}

func reverseArray[E any](arr []E) []E {
	mid := len(arr) / 2
	for i := 0; i < mid; i++ {
		j := len(arr) - i - 1
		arr[i], arr[j] = arr[j], arr[i]
	}
	return arr
}
