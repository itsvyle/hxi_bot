package config

import (
	"log/slog"
	"os"

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
