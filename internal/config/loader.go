package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"

	"nuimanbot/internal/domain"
)

const (
	defaultConfigFileName = "config"
	configFileType        = "yaml"
	envPrefix             = "NUIMANBOT"
	encryptionKeyEnv      = "NUIMANBOT_ENCRYPTION_KEY"
)

// LoadConfig loads the application configuration from specified paths and environment variables.
func LoadConfig(configPaths ...string) (*NuimanBotConfig, error) {
	// Validate encryption key is set
	if os.Getenv(encryptionKeyEnv) == "" {
		return nil, fmt.Errorf("%s is not set in environment", encryptionKeyEnv)
	}

	// Load .env file
	_ = godotenv.Load()

	v := viper.New()
	v.SetConfigName(defaultConfigFileName)
	v.SetConfigType(configFileType)

	// Add config paths, starting with the current directory
	v.AddConfigPath(".")
	for _, path := range configPaths {
		if path != "" {
			v.AddConfigPath(path)
		}
	}

	// Attempt to read the config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// If config file not found, proceed, assuming env vars will provide config
		fmt.Println("No config file found, loading configuration from environment variables only.")
	} else {
		fmt.Printf("Config file used: %s\n", v.ConfigFileUsed())
	}

	var cfg NuimanBotConfig
	// Debug: Print all settings before unmarshaling
	fmt.Printf("Viper settings before unmarshal: %+v\n", v.AllSettings())

	// Get all settings and remove providers (we'll handle manually)
	allSettings := v.AllSettings()
	if llmSettings, ok := allSettings["llm"].(map[string]interface{}); ok {
		delete(llmSettings, "providers")
	}

	decoderConfig := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   &cfg,
		TagName:  "yaml",
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create mapstructure decoder: %w", err)
	}

	if err := decoder.Decode(allSettings); err != nil {
		return nil, fmt.Errorf("failed to decode viper settings into config struct: %w", err)
	}

	// Apply environment variable overrides (env vars take precedence over file)
	applyEnvOverrides(&cfg)

	// Manual unmarshalling for llm.providers from file
	cfg.LLM.Providers = []LLMProviderConfig{}
	if v.IsSet("llm.providers") {
		if providers, ok := v.Get("llm.providers").([]interface{}); ok {
			for _, provider := range providers {
				if p, ok := provider.(map[string]interface{}); ok {
					var providerCfg LLMProviderConfig
					if id, ok := p["id"].(string); ok {
						providerCfg.ID = id
					}
					if provType, ok := p["type"].(string); ok {
						providerCfg.Type = domain.LLMProvider(provType)
					}
					if apiKey, ok := p["api_key"].(string); ok {
						providerCfg.APIKey = domain.NewSecureStringFromString(apiKey)
					}
					if baseURL, ok := p["base_url"].(string); ok {
						providerCfg.BaseURL = baseURL
					}
					if name, ok := p["name"].(string); ok {
						providerCfg.Name = name
					}
					cfg.LLM.Providers = append(cfg.LLM.Providers, providerCfg)
				}
			}
		}
	}

	// Load providers from environment variables
	loadProvidersFromEnv(&cfg)

	// Manually populate SecureString fields from viper
	if v.IsSet("llm.anthropic.api_key") {
		cfg.LLM.Anthropic.APIKey = domain.NewSecureStringFromString(v.GetString("llm.anthropic.api_key"))
	}
	if v.IsSet("llm.openai.api_key") {
		cfg.LLM.OpenAI.APIKey = domain.NewSecureStringFromString(v.GetString("llm.openai.api_key"))
	}
	if v.IsSet("gateways.telegram.token") {
		cfg.Gateways.Telegram.Token = domain.NewSecureStringFromString(v.GetString("gateways.telegram.token"))
	}
	if v.IsSet("gateways.slack.bot_token") {
		cfg.Gateways.Slack.BotToken = domain.NewSecureStringFromString(v.GetString("gateways.slack.bot_token"))
	}
	if v.IsSet("gateways.slack.app_token") {
		cfg.Gateways.Slack.AppToken = domain.NewSecureStringFromString(v.GetString("gateways.slack.app_token"))
	}
	if v.IsSet("external_api.openai.api_key") {
		cfg.ExternalAPI.OpenAI.APIKey = domain.NewSecureStringFromString(v.GetString("external_api.openai.api_key"))
	}
	if v.IsSet("external_api.rest.api_key") {
		cfg.ExternalAPI.REST.APIKey = domain.NewSecureStringFromString(v.GetString("external_api.rest.api_key"))
	}

	// Load skills from environment variables
	loadSkillsFromEnv(&cfg)

	return &cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the config.
// Environment variables take precedence over file values.
func applyEnvOverrides(cfg *NuimanBotConfig) {
	// Server config
	if val := os.Getenv("NUIMANBOT_SERVER_LOGLEVEL"); val != "" {
		cfg.Server.LogLevel = val
	}
	if val := os.Getenv("NUIMANBOT_SERVER_DEBUG"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			cfg.Server.Debug = b
		}
	}

	// Security config
	if val := os.Getenv("NUIMANBOT_SECURITY_INPUTMAXLENGTH"); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			cfg.Security.InputMaxLength = i
		}
	}
	if val := os.Getenv("NUIMANBOT_SECURITY_TOKENROTATIONHOURS"); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			cfg.Security.TokenRotationHours = i
		}
	}
	if val := os.Getenv("NUIMANBOT_ENCRYPTION_KEY"); val != "" {
		cfg.Security.EncryptionKey = val
	}

	// LLM config
	if val := os.Getenv("NUIMANBOT_LLM_DEFAULTMODEL_PRIMARY"); val != "" {
		cfg.LLM.DefaultModel.Primary = val
	}

	// Gateway config
	if val := os.Getenv("NUIMANBOT_GATEWAYS_CLI_DEBUGMODE"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			cfg.Gateways.CLI.DebugMode = b
		}
	}

	// MCP config
	if val := os.Getenv("NUIMANBOT_MCP_CLIENT_TIMEOUT"); val != "" {
		cfg.MCP.Client.Timeout = val
	}
}

// loadProvidersFromEnv loads LLM provider configurations from environment variables.
func loadProvidersFromEnv(cfg *NuimanBotConfig) {
	// Check for providers in env vars (NUIMANBOT_LLM_PROVIDERS_0_ID, etc.)
	for i := 0; i < 10; i++ {
		idKey := fmt.Sprintf("NUIMANBOT_LLM_PROVIDERS_%d_ID", i)
		typeKey := fmt.Sprintf("NUIMANBOT_LLM_PROVIDERS_%d_TYPE", i)
		apiKeyKey := fmt.Sprintf("NUIMANBOT_LLM_PROVIDERS_%d_APIKEY", i)

		id := os.Getenv(idKey)
		if id == "" {
			// If ID is not set, skip this index
			continue
		}

		providerType := os.Getenv(typeKey)
		apiKey := os.Getenv(apiKeyKey)

		// Check if this provider already exists in config (from file)
		found := false
		for j, existing := range cfg.LLM.Providers {
			if existing.ID == id {
				// Override with env var values
				if providerType != "" {
					cfg.LLM.Providers[j].Type = domain.LLMProvider(providerType)
				}
				if apiKey != "" {
					cfg.LLM.Providers[j].APIKey = domain.NewSecureStringFromString(apiKey)
				}
				found = true
				break
			}
		}

		// If not found, add new provider
		if !found {
			provider := LLMProviderConfig{
				ID:   id,
				Type: domain.LLMProvider(providerType),
			}
			if apiKey != "" {
				provider.APIKey = domain.NewSecureStringFromString(apiKey)
			}
			cfg.LLM.Providers = append(cfg.LLM.Providers, provider)
		}
	}
}

// loadSkillsFromEnv loads skill configurations from environment variables.
func loadSkillsFromEnv(cfg *NuimanBotConfig) {
	// Initialize map if not exists
	if cfg.Skills.Entries == nil {
		cfg.Skills.Entries = make(map[string]SkillConfig)
	}

	// Check for calculator skill API key
	if apiKey := os.Getenv("NUIMANBOT_SKILLS_ENTRIES_CALCULATOR_APIKEY"); apiKey != "" {
		skillCfg := cfg.Skills.Entries["calculator"]
		skillCfg.APIKey = domain.NewSecureStringFromString(apiKey)
		cfg.Skills.Entries["calculator"] = skillCfg
	}

	// Add more skills as needed
}
