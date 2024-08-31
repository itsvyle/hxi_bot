package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/itsvyle/hxi_bot/config"
	"golang.org/x/time/rate"

	openai "github.com/sashabaranov/go-openai"
)

type ChatBot interface {
	Query(messages *[]openai.ChatCompletionMessage) (string, error)
}

const aiMaxCacheLength = 100

type AIChatbotCachedMessage struct {
	ID         string
	Bot        bool
	ChannelID  string
	Content    string
	References string
}

type ServiceAiChatBot struct {
	config               *config.ConfigSchemaJsonAiChatServicesElem
	chatBot              ChatBot
	myID                 string
	botTrigger           string
	cache                []*AIChatbotCachedMessage
	emojis               map[string]string
	ongoingConversations []*BotToBotConvo
	rateLimitter         *rate.Limiter
}

func CreateNewServiceAiChatBot(config *config.ConfigSchemaJsonAiChatServicesElem) *ServiceAiChatBot {
	return &ServiceAiChatBot{
		config: config,
	}
}

type BotToBotConvo struct {
	SelfStarted   bool      `json:"selfStarted"`
	OtherBotID    string    `json:"otherBotID"`
	ChannelID     string    `json:"channelID"`
	TotalAmount   int       `json:"totalAmount"`
	CurrentAmount int       `json:"currentAmount"`
	StartedAt     time.Time `json:"startedAt"`
}

func (s *ServiceAiChatBot) InitAiChatBot(discordSession *discordgo.Session) {
	s.chatBot = CreateNewOpenAIChatBot(s.config.OpenAIAPiKey, s.config.OpenAIModelName, s.config.MaxTokens, s.config.Temperature)

	s.myID = discordSession.State.User.ID
	s.botTrigger = fmt.Sprintf("<@%s>", discordSession.State.User.ID)

	s.ongoingConversations = make([]*BotToBotConvo, 0)

	emojisWithIDPattern := regexp.MustCompile(`<:(\w+):\d+>`)
	emojisWithoutIDPattern := regexp.MustCompile(`:(\w+):`)

	s.InitEmojis(discordSession)

	var botRoles []string
	if s.config.GuildId != nil {
		botMember, err := discordSession.GuildMember(*s.config.GuildId, s.myID)
		if err == nil {
			botRoles = make([]string, len(botMember.Roles))
			copy(botRoles, botMember.Roles)
			slog.With("roles", botRoles).Info("Initialized bot roles")
		}
	}

	discordSession.AddHandler(func(session *discordgo.Session, message *discordgo.MessageCreate) {
		if message.Author.ID == session.State.User.ID {
			return
		}
		if !message.Author.Bot && !strings.Contains(message.Content, s.botTrigger) {
			mentionsMe := false
			for _, user := range message.Mentions {
				if user.ID == session.State.User.ID {
					mentionsMe = true
					break
				}
			}
			if !mentionsMe {
				for _, role := range message.MentionRoles {
					for _, botRole := range botRoles {
						if role == botRole {
							mentionsMe = true
							break
						}
					}
				}
				if !mentionsMe {
					return
				}
			}
		}

		slog.Info("------------------------------------------------")

		if message.Author.Bot {
			if strings.Contains(message.Content, fmt.Sprintf("<@%s> !start", s.myID)) {
				for _, convo := range s.ongoingConversations {
					if convo.OtherBotID == message.Author.ID {
						_, _ = session.ChannelMessageSendReply(message.ChannelID, "Already in a conversation with this bot", message.Reference())
						slog.Info("Already in a conversation with this bot", "botID", message.Author.ID)
						return
					}
				}

				splits := strings.Split(message.Content, "!start")
				if len(splits) < 2 {
					return
				}
				amount := 0
				_, err := fmt.Sscanf(strings.TrimSpace(splits[1]), "%d", &amount)
				if err != nil {
					slog.With("error", err).Error("Error parsing amount")
					return
				}
				slog.Info("Received conversation start request", "botID", message.Author.ID, "amount", amount)
				s.ongoingConversations = append(s.ongoingConversations, &BotToBotConvo{
					SelfStarted:   false,
					OtherBotID:    message.Author.ID,
					ChannelID:     message.ChannelID,
					TotalAmount:   amount,
					CurrentAmount: 0,
					StartedAt:     time.Now(),
				})
				return
			}
			foundConvo := false
			for _, convo := range s.ongoingConversations {
				if convo.OtherBotID == message.Author.ID && convo.ChannelID == message.ChannelID {
					convo.CurrentAmount++
					if convo.CurrentAmount >= convo.TotalAmount {
						slog.Info("Conversation ended", "botID", message.Author.ID, "currentAmount", convo.CurrentAmount, "totalAmount", convo.TotalAmount)
						// Delete this conversation
						o := make([]*BotToBotConvo, int(math.Max(0, float64(len(s.ongoingConversations)-1))))
						j := 0
						for _, c := range s.ongoingConversations {
							if c.OtherBotID != message.Author.ID {
								o[j] = c
								j++
							}
						}
						s.ongoingConversations = o
						return
					}
					foundConvo = true
					slog.Info("Continuing conversation", "botID", message.Author.ID, "currentAmount", convo.CurrentAmount, "totalAmount", convo.TotalAmount)
					break
				}
			}
			if !foundConvo {
				return
			}
			time.Sleep(time.Duration(s.config.AutoConvosMessageDelay) * time.Millisecond)
		}

		if strings.Contains(message.Content, "!kill") {
			for _, id := range s.config.Killers {
				if message.Author.ID == id {
					slog.Info("Killed by user", "userID", id)
					_, _ = session.ChannelMessageSendReply(message.ChannelID, "Je meurs (pour de vrai)....", message.Reference())
					os.Exit(0)
					return
				}
			}
		}
		if strings.Contains(message.Content, "!stop") {
			s.ongoingConversations = s.ongoingConversations[:0]
			_, _ = session.ChannelMessageSendReply(message.ChannelID, "Stopped all ongoing conversations", message.Reference())
			return
		}

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

		res, err := s.chatBot.Query(&chain)
		if err != nil {
			_, _ = session.ChannelMessageSendReply(message.ChannelID, "Error getting response from AI", message.Reference())
			slog.With("error", err).Error("Error getting response from AI")
			return
		}

		res = strings.Replace(res, "@everyone", "`@everyone`", -1)
		res = strings.Replace(res, "@here", "`@here`", -1)

		res = emojisWithIDPattern.ReplaceAllString(res, ":$1:") // Remove all emoji IDs, to add them back
		matches := emojisWithoutIDPattern.FindAllString(res, -1)
		for _, match := range matches {
			emojiName := match[1 : len(match)-1]
			if emojiID, ok := s.emojis[strings.ToLower(emojiName)]; ok {
				res = strings.Replace(res, match, fmt.Sprintf("<:%s:%s>", emojiName, emojiID), 1)
			}
		}

		slog.Info("AI response", "response", res)

		newM, err := session.ChannelMessageSendReply(message.ChannelID, res, message.Reference())
		if err != nil {
			slog.With("error", err, "channelID", message.ChannelID).Error("Error sending message to channel")
			return
		}
		s.appendToCache(newM)
	})

	discordSession.AddHandler(func(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
		if interaction.Type != discordgo.InteractionApplicationCommand {
			return
		}
		cmdName := interaction.ApplicationCommandData().Name
		if cmdName == "chatbotconfig" {
			s.sendConfig(session, interaction)
			return
		}

		if cmdName != "convo" {
			_ = session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Invalid command",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		otherBot := interaction.ApplicationCommandData().Options[0].UserValue(session)
		amount := interaction.ApplicationCommandData().Options[1].IntValue()
		firstMessage := interaction.ApplicationCommandData().Options[2].StringValue()
		channelID := interaction.ChannelID

		if !otherBot.Bot {
			_ = session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "The specified user isn't a bot",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}
		if amount < 1 || amount > 25 {
			_ = session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Amount must be between 1 and 25",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		for _, convo := range s.ongoingConversations {
			if convo.OtherBotID == otherBot.ID {
				_ = session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "There is already an ongoing conversation with this bot",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}
		}

		slog.Info("Starting conversation", "botID", otherBot.ID, "amount", amount, "firstMessage", firstMessage)

		s.ongoingConversations = append(s.ongoingConversations, &BotToBotConvo{
			SelfStarted:   true,
			OtherBotID:    otherBot.ID,
			ChannelID:     channelID,
			TotalAmount:   int(amount),
			CurrentAmount: 1,
			StartedAt:     time.Now(),
		})

		startMessage, err := session.ChannelMessageSend(channelID, fmt.Sprintf("<@%s> !start %d", otherBot.ID, amount))
		if err != nil {
			slog.With("error", err).Error("Error staring conversation")
			_ = session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Error starting conversation",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		time.Sleep(3 * time.Second)

		_ = session.ChannelMessageDelete(channelID, startMessage.ID)

		_, err = session.ChannelMessageSend(channelID, fmt.Sprintf("<@%s> %s", otherBot.ID, firstMessage))
		if err != nil {
			slog.With("error", err).Error("Error staring conversation")
			_ = session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Error starting conversation",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		_ = session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Conversation started",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})

	})

	slog.Info("Initialized AI chat bot", "botName", s.config.BotName, "prompt", s.config.Prompt)
}

func (s *ServiceAiChatBot) processMessageInputContent(message *discordgo.Message) string {
	ret := strings.Replace(message.Content, s.botTrigger, "@"+s.config.BotName, 1)
	for _, mention := range message.Mentions {
		ret = strings.Replace(ret, fmt.Sprintf("<@%s>", mention.ID), "@"+mention.Username, 1)
	}
	return ret
}

func (s *ServiceAiChatBot) appendToCache(message *discordgo.Message) *AIChatbotCachedMessage {
	var r string
	if message.MessageReference != nil {
		r = message.MessageReference.MessageID
	}
	m := &AIChatbotCachedMessage{
		ID:         message.ID,
		ChannelID:  message.ChannelID,
		Content:    s.processMessageInputContent(message),
		References: r,
		Bot:        message.Author.ID == s.myID,
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
		Content: s.processMessageInputContent(message),
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
		m = s.getMessageCached(m.References, message.ChannelID, discordSession)
		if m == nil {
			return reverseArray(r)
		}

		role := openai.ChatMessageRoleUser
		if m.Bot {
			role = openai.ChatMessageRoleAssistant
		}
		r = append(r, openai.ChatCompletionMessage{
			Role:    role,
			Content: m.Content,
		})
		i++
	}
	return reverseArray(r)
}

func (s *ServiceAiChatBot) getMessageCached(messageID string, channelID string, discordSession *discordgo.Session) *AIChatbotCachedMessage {
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

func (s *ServiceAiChatBot) InitCommands(session *discordgo.Session) {
	payload := []*discordgo.ApplicationCommand{
		{
			Name:        "chatbotconfig",
			Description: "Get the configuration of the chatbot",
			Type:        discordgo.ChatApplicationCommand,
		},
	}

	if s.config.ActivateAutoConvos {
		payload = append(payload, &discordgo.ApplicationCommand{
			Name:        "convo",
			Description: "Start an auto conversation between this bot and another",
			Type:        discordgo.ChatApplicationCommand,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "otherbot",
					Description: "The other bot to have a conversation with",
					Type:        discordgo.ApplicationCommandOptionMentionable,
					Required:    true,
				},
				{
					Name:        "amount",
					Description: "The amount of messages to send (for each bot, so total messages will be double this)",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
				},
				{
					Name:        "firstmessage",
					Description: "The first message to send",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
			},
		})
	}

	_, err := session.ApplicationCommandBulkOverwrite(session.State.User.ID, "", payload)
	if err != nil {
		slog.With("error", err).Error("Error initializing commands")
		panic(err)
	}
	slog.With("commandsCount", len(payload)).Info("Initialized commands")
}

func (s *ServiceAiChatBot) InitEmojis(session *discordgo.Session) {
	if s.config.GuildId == nil {
		return
	}
	s.emojis = make(map[string]string)
	emojis, err := session.GuildEmojis(*s.config.GuildId)
	if err != nil {
		slog.With("error", err).Error("Error getting emojis")
		return
	}

	for _, emoji := range emojis {
		s.emojis[strings.ToLower(emoji.Name)] = emoji.ID
	}
	slog.With("emojisCount", len(s.emojis), "guildID", s.config.GuildId).Info("Initialized emojis")
}

type ChatbotPublicConfig struct {
	BotName                string  `json:"botName"`
	Prompt                 string  `json:"prompt"`
	MaxTokens              int     `json:"maxTokens"`
	Temperature            float64 `json:"temperature"`
	AutoConvosMessageDelay int     `json:"autoConvosMessageDelay"`
	ActivateAutoConvos     bool    `json:"activateAutoConvos"`
	MaxContextSize         int     `json:"maxContextSize"`
	EmojisGuildID          *string `json:"emojisGuildID"`
	Conversations          []*BotToBotConvo
}

func (s *ServiceAiChatBot) sendConfig(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	res := ""
	config := &ChatbotPublicConfig{
		BotName:                s.config.BotName,
		Prompt:                 s.config.Prompt,
		MaxTokens:              s.config.MaxTokens,
		Temperature:            s.config.Temperature,
		AutoConvosMessageDelay: s.config.AutoConvosMessageDelay,
		ActivateAutoConvos:     s.config.ActivateAutoConvos,
		MaxContextSize:         s.config.MaxContextSize,
		EmojisGuildID:          s.config.GuildId,
		Conversations:          s.ongoingConversations,
	}
	configBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		slog.With("error", err).Error("Error marshalling config")
		res = "Error getting config"
	} else {
		res = "```json\n" + string(configBytes) + "\n```"
	}
	_ = session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: res,
		},
	})
}
