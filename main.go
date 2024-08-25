package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		panic("Error loading .env file")
	}
}

func main() {
	slog.Info("Starting up...")

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		panic("BOT_TOKEN is not set")
	}

	discordSession, err := discordgo.New("Bot " + botToken)
	if err != nil {
		slog.With("error", err).Error("Error creating Discord session")
		panic("Error creating Discord session")
	}

	discordSession.ShouldReconnectOnError = true
	discordSession.Identify.Intents = discordgo.IntentsGuildMessages

	discordSession.AddHandler(readyBot)

	InitWatchChannelForThread(discordSession)

	err = discordSession.Open()
	if err != nil {
		slog.With("error", err).Error("Error logging in to the discord session. Check that token is valid.")
		panic("Error logging in to the discord session. Check that token is valid.")
	}

	defer slog.Info("Bot disconnecting...")
	defer discordSession.Close()

	slog.Info("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

func readyBot(session *discordgo.Session, _ *discordgo.Ready) {
	slog.Info("Logged in as: %v#%v", session.State.User.Username, session.State.User.Discriminator)
}
