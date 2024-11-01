package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

/*
To generate the json schema, run the following commands:
- Check that the https://github.com/omissis/go-jsonschema is installed, `go install github.com/atombender/go-jsonschema@latest`
- Run the command: `go-jsonschema -p config -o "config/config-schema.go" config/config.schema.json`
*/

var Config ConfigSchemaJson

func InitConfig() {
	_ = godotenv.Load()

	filePath := "configs/config.json"
	if f := os.Getenv("CONFIG_PATH"); f != "" {
		filePath = f
	}

	b, err := os.ReadFile(filePath)
	if err != nil {
		slog.With("error", err).Error("Error reading config file")
		panic("Error reading config file")
	}

	jsonContent := string(b)
	jsonContent = os.ExpandEnv(jsonContent)

	err = Config.UnmarshalJSON([]byte(jsonContent))
	if err != nil {
		slog.With("error", err).Error("Error unmarshalling config file")
		panic("Error unmarshalling config file")
	}
}

func (r *ConfigSchemaJsonGayGPTServicesElemReactTo) Init() {
	for key, value := range *r {
		value.TotalWeight = value.EmptyWeight
		value.Weights = make([]int, len(value.Emojis)+1)

		value.Weights[0] = value.EmptyWeight
		for i, emoji := range value.Emojis {
			value.TotalWeight += emoji.Weight
			value.Weights[i+1] = value.TotalWeight

			emoji.LastUsed = new(time.Time)

			value.Emojis[i] = emoji
		}

		(*r)[key] = value
	}

	data, err := json.MarshalIndent(*r, "", "  ")
	if err != nil {
		// Handle error
	}
	fmt.Println(string(data))
}

func (r *ConfigSchemaJsonGayGPTServicesElemReactTo) ReactWithEmoji(session *discordgo.Session, message *discordgo.MessageCreate) {
	reaction, ok := (*r)[message.Author.ID]
	if !ok {
		return
	}
	if reaction.TotalWeight == 0 {
		return
	}

	if len(reaction.ExcludeChannels) > 0 {
		for _, channelID := range reaction.ExcludeChannels {
			if channelID == message.ChannelID {
				return
			}
		}
	}

	randomNumber := rand.Intn(reaction.TotalWeight)
	// fmt.Println("randomNumber: ", randomNumber)
	for i, weight := range reaction.Weights {
		if randomNumber < weight {
			if i == 0 {
				return
			}

			emojiChoice := &reaction.Emojis[i-1]
			if emojiChoice.Cooldown > 0 {
				if message.Timestamp.Add(-1 * time.Duration(emojiChoice.Cooldown) * time.Second).Before(*emojiChoice.LastUsed) {
					fmt.Printf("emojiChoice: %+v ON CD\n", emojiChoice)

					return
				}
			}
			emojiChoice.LastUsed = &message.Timestamp

			for _, emojiID := range emojiChoice.Emojis {
				err := session.MessageReactionAdd(message.ChannelID, message.ID, emojiID)
				if err != nil {
					slog.With("error", err, "messageID", message.ID).Error("Error adding reaction to message")
				}
			}
			return
		}
	}
}

/*
if reaction, ok := s.config.ReactTo[message.Author.ID]; ok {
			if len(reaction.EmojiIds) == 0 {
				return
			}

			if len(reaction.ExcludeChannels) > 0 {
				for _, channelID := range reaction.ExcludeChannels {
					if channelID == message.ChannelID {
						return
					}
				}
			}

			for _, emojiID := range reaction.EmojiIds {
				err := session.MessageReactionAdd(message.ChannelID, message.ID, emojiID)
				if err != nil {
					slog.With("error", err, "messageID", message.ID).Error("Error adding reaction to message")
				}
			}
		}

*/
