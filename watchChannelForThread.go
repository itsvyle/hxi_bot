package main

import (
	"log/slog"
	"os"

	"github.com/bwmarrin/discordgo"
)

func InitWatchChannelForThread(discordSession *discordgo.Session) {
	channelID := os.Getenv("THREADS_CHANNEL_ID")
	if channelID == "" {
		panic("THREADS_CHANNEL_ID is not set")
	}

	discordSession.AddHandler(func(session *discordgo.Session, message *discordgo.MessageCreate) {
		if message.ChannelID != channelID {
			return
		}
		if message.Author.ID == session.State.User.ID {
			return
		}

		thread, err := session.MessageThreadStartComplex(message.ChannelID, message.ID, &discordgo.ThreadStart{
			Name:                message.Content,
			AutoArchiveDuration: 24 * 60,
		})
		if err != nil {
			slog.With("error", err).Error("Error creating thread")
			return
		}

		_, err = session.ChannelMessageSendComplex(thread.ID, &discordgo.MessageSend{
			Content: "Clique sur le boutton pour changer le nom du fil!",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Style:    discordgo.SuccessButton,
							Label:    "Renommer le fil",
							CustomID: "thread_rename",
						},
					},
				},
			},
		})
		if err != nil {
			slog.With("error", err, "threadID", thread.ID).Error("Error sending message to thread")
			return
		}
	})

	// Handle button clicks
	discordSession.AddHandler(func(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
		if interaction.Type != discordgo.InteractionMessageComponent {
			return
		}

		customID := interaction.MessageComponentData().CustomID
		if customID != "thread_rename" {
			return
		}

		sourceChannel, err := session.Channel(interaction.ChannelID)
		if err != nil {
			slog.With("error", err, "channelID", interaction.ChannelID).Error("Error getting source channel")
			return
		}

		err = session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: "modal_thread_rename_" + interaction.ChannelID, // Unique ID so the previous value doesn't automatically get filled in
				Title:    "Renommer le thread",
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID:    "name",
								Label:       "Titre du thread",
								Placeholder: "Nouveau titre:",
								Style:       discordgo.TextInputParagraph,
								Required:    true,
								Value:       sourceChannel.Name,
							},
						},
					},
				},
			},
		})
		if err != nil {
			slog.With("error", err).Error("Error presenting rename modal for user")
			return
		}
	})

	// Handle modal submit
	discordSession.AddHandler(func(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
		if interaction.Type != discordgo.InteractionModalSubmit {
			return
		}

		customID := interaction.ModalSubmitData().CustomID
		if customID != "modal_thread_rename_"+interaction.ChannelID {
			return
		}

		newName := interaction.ModalSubmitData().Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
		if newName == "" {
			_ = session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Le nom du thread n'a pas été modifié.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}
		_, err := session.ChannelEdit(interaction.ChannelID, &discordgo.ChannelEdit{
			Name: newName,
		})
		if err != nil {
			slog.With("error", err, "channelID", interaction.ChannelID).Error("Error renaming thread")
			_ = session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Le nom du thread n'a pas pu etre modifié, il y a eu une erreur.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}
		_ = session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Le nom du thread a été modifié avec succès.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	})
}
