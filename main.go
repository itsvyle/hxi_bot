package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/itsvyle/hxi_bot/config"
)

func init() {
	config.InitConfig()
}

func main() {
	slog.Info("Starting up...")

	botToken := config.Config.BotToken

	discordSession, err := discordgo.New("Bot " + botToken)
	if err != nil {
		slog.With("error", err).Error("Error creating Discord session")
		panic("Error creating Discord session")
	}

	discordSession.ShouldReconnectOnError = true
	discordSession.Identify.Intents = discordgo.IntentsGuildMessages

	discordSession.AddHandler(readyBot)

	err = discordSession.Open()
	if err != nil {
		slog.With("error", err).Error("Error logging in to the discord session. Check that token is valid.")
		panic("Error logging in to the discord session. Check that token is valid.")
	}

	defer slog.Info("Bot disconnecting...")
	defer discordSession.Close()

	channelWatchers := make([]*ServiceWatchChannels, len(config.Config.ChannelThreadsWatcherServices))
	aiChatsBots := make([]*ServiceAiChatBot, len(config.Config.AiChatServices))

	for i, service := range config.Config.ChannelThreadsWatcherServices {
		channelWatchers[i] = CreateNewServiceWatchChannels(service)
		channelWatchers[i].InitWatchChannelForThread(discordSession)
	}
	for i, service := range config.Config.AiChatServices {
		aiChatsBots[i] = CreateNewServiceAiChatBot(&service)
		aiChatsBots[i].InitAiChatBot(discordSession)
	}

	slog.Info("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

func readyBot(session *discordgo.Session, _ *discordgo.Ready) {
	slog.Info("Logged in", "username", session.State.User.Username, "discrim", session.State.User.Discriminator)
}
