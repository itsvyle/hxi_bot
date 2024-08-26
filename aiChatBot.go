package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/itsvyle/hxi_bot/config"

	openai "github.com/sashabaranov/go-openai"
)

const aiMaxCacheLength = 100

type AIChatbotCachedMessage struct {
	ID         string
	Bot        bool
	ChannelID  string
	Content    string
	References string
}

type ServiceAiChatBot struct {
	config       *config.ConfigSchemaJsonAiChatServicesElem
	openAIClient *openai.Client
	botTrigger   string
	cache        []*AIChatbotCachedMessage
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
		if message.Author.ID == session.State.User.ID || message.Author.Bot {
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

		if strings.Contains(message.Content, "kill") {
			for _, id := range s.config.Killers {
				if message.Author.ID == id {
					_, _ = session.ChannelMessageSendReply(message.ChannelID, "Je meurs (pour de vrai)....", message.Reference())
					os.Exit(0)
					return
				}
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

		newM, err := session.ChannelMessageSendReply(message.ChannelID, res, message.Reference())
		if err != nil {
			slog.With("error", err, "channelID", message.ChannelID).Error("Error sending message to channel")
			return
		}
		s.appendToCache(newM)
	})

	slog.Info("Initialized AI chat bot", "botName", s.config.BotName, "prompt", s.config.Prompt)
}

func (s *ServiceAiChatBot) processMessageContentInput(message string) string {
	return strings.Replace(message, s.botTrigger, "@"+s.config.BotName, 1)
}

func (s *ServiceAiChatBot) appendToCache(message *discordgo.Message) *AIChatbotCachedMessage {
	var r string
	if message.MessageReference != nil {
		r = message.MessageReference.MessageID
	}
	m := &AIChatbotCachedMessage{
		ID:         message.ID,
		ChannelID:  message.ChannelID,
		Content:    message.Content,
		References: r,
	}
	s.cache = append(s.cache, m)
	if len(s.cache) > aiMaxCacheLength {
		slog.Info("Cache is full, removing half of the cache")
		s.cache = s.cache[aiMaxCacheLength/2:]
	}
	return m
}

func (s *ServiceAiChatBot) getMessageChain(discordSession *discordgo.Session, message *discordgo.Message) []openai.ChatCompletionMessage {
	r := make([]openai.ChatCompletionMessage, 1, s.config.MaxContextSize)
	r[0] = openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: s.processMessageContentInput(message.Content),
	}
	i := 1

	m := &AIChatbotCachedMessage{
		ID:         message.ID,
		ChannelID:  message.ChannelID,
		Content:    message.Content,
		Bot:        false,
		References: "",
	}
	if message.MessageReference != nil {
		m.References = message.MessageReference.MessageID
	}

	for m.References != "" && i < s.config.MaxContextSize {
		m = s.getMessageContent(m.References, message.ChannelID, discordSession)
		if m == nil {
			return reverseArray(r)
		}

		role := openai.ChatMessageRoleUser
		if m.Bot {
			role = openai.ChatMessageRoleAssistant
		}
		r = append(r, openai.ChatCompletionMessage{
			Role:    role,
			Content: s.processMessageContentInput(m.Content),
		})
		i++
	}
	return reverseArray(r)
}

func (s *ServiceAiChatBot) getMessageContent(messageID string, channelID string, discordSession *discordgo.Session) *AIChatbotCachedMessage {
	for i := len(s.cache) - 1; i >= 0; i-- {
		if s.cache[i].ID == messageID && s.cache[i].ChannelID == channelID {
			return s.cache[i]
		}
	}
	slog.Info("Cache miss", "messageID", messageID, "channelID", channelID)
	message, err := discordSession.ChannelMessage(channelID, messageID)
	if err != nil {
		slog.With("error", err).Error("Error getting message content")
		return nil
	}
	return s.appendToCache(message)
}

func reverseArray[E any](arr []E) []E {
	mid := len(arr) / 2
	for i := 0; i < mid; i++ {
		j := len(arr) - i - 1
		arr[i], arr[j] = arr[j], arr[i]
	}
	return arr
}
