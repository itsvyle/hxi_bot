package main

import (
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/itsvyle/hxi_bot/config"
)

type ServiceGayGPT struct {
	config         *config.ConfigSchemaJsonGayGPTServicesElem
	discordSession *discordgo.Session
}

func CreateNewServiceGayGPT(botConfig *config.ConfigSchemaJsonGayGPTServicesElem) *ServiceGayGPT {

	if botConfig.ReactTo == nil {
		botConfig.ReactTo = make(config.ConfigSchemaJsonGayGPTServicesElemReactTo)
	}
	if botConfig.AutoReactTrigger == nil {
		botConfig.AutoReactTrigger = make(config.ConfigSchemaJsonGayGPTServicesElemAutoReactTrigger)
	}
	botConfig.ReactTo.Init()

	return &ServiceGayGPT{
		config: botConfig,
	}
}

func (s *ServiceGayGPT) InitGayGPT() {
	var err error
	s.discordSession, err = discordgo.New("Bot " + s.config.BotToken)
	if err != nil {
		slog.With("error", err, "gayGPT", true).Error("Error creating Discord session")
		panic("Error creating Discord session")
	}

	s.discordSession.ShouldReconnectOnError = true
	s.discordSession.Identify.Intents = discordgo.IntentsGuildMessages

	s.discordSession.AddHandler(readyBot)

	err = s.discordSession.Open()
	if err != nil {
		slog.With("error", err, "gayGPT", true).Error("Error logging in to the discord session. Check that token is valid.")
		panic("Error logging in to the discord session. Check that token is valid.")
	}

	var botRoles []string
	if s.config.GuildId != nil {
		botMember, err := s.discordSession.GuildMember(*s.config.GuildId, s.discordSession.State.User.ID)
		if err == nil {
			botRoles = make([]string, len(botMember.Roles))
			copy(botRoles, botMember.Roles)
			slog.With("roles", botRoles, "gayGPT", true).Info("Initialized bot roles")
		}
	}

	rand.Seed(time.Now().UnixNano())

	s.discordSession.AddHandler(func(session *discordgo.Session, message *discordgo.MessageCreate) {
		if message.Author.ID == session.State.User.ID {
			return
		}
		if message.Author.Bot {
			return
		}

		s.config.ReactTo.ReactWithEmoji(session, message)

		if message.Content != "" {
			c := strings.ToLower(message.Content)
			for trigger, reactWith := range s.config.AutoReactTrigger {
				if strings.Contains(c, trigger) {
					session.MessageReactionAdd(message.ChannelID, message.ID, reactWith)
				}
			}
		}

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

		randIndex := rand.Intn(len(s.config.PossibleAnswers))
		answer := s.config.PossibleAnswers[randIndex]

		if message.ReferencedMessage != nil && message.ReferencedMessage.Author.ID != session.State.User.ID {
			_, err = session.ChannelMessageSendReply(message.ChannelID, answer, message.ReferencedMessage.Reference())

			err2 := session.ChannelMessageDelete(message.ChannelID, message.ID)
			if err2 != nil {
				slog.With("error", err2, "channelID", message.ChannelID).Error("Error deleting message")
			}
		} else {
			_, err = session.ChannelMessageSendReply(message.ChannelID, answer, message.Reference())
		}
		if err != nil {
			slog.With("error", err, "channelID", message.ChannelID).Error("Error sending message to channel")
			return
		}
	})
}

func (s *ServiceGayGPT) Close() {
	slog.With("gayGPT", true).Info("Closing connection...")
	s.discordSession.Close()
}
