package config

import (
	"fmt"
	"strings"

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

	// Read environment variables
	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv() // Enable automatic env var reading

	// Explicitly bind environment variables for top-level keys
	v.BindEnv("server.log_level")
	v.BindEnv("server.debug")
	v.BindEnv("security.input_max_length")
	v.BindEnv("security.token_rotation_hours")
	v.BindEnv("security.encryption_key")
	v.BindEnv("llm.default_model.primary")
	v.BindEnv("gateways.cli.debug_mode")
	v.BindEnv("mcp.client.timeout") // Now string
	// Bind for LLM providers array
	for i := 0; i < 5; i++ { // Assuming max 5 providers for simplicity in binding
		v.BindEnv(fmt.Sprintf("llm.providers.%d.id", i))
		v.BindEnv(fmt.Sprintf("llm.providers.%d.type", i))
		v.BindEnv(fmt.Sprintf("llm.providers.%d.api_key", i))
	}
	// Bind for skills map (example for calculator skill)
	v.BindEnv("skills.entries.calculator.api_key")
	v.BindEnv("externalapi.openai.api_key")
	v.BindEnv("externalapi.rest.api_key")
	v.BindEnv("tools.web_search.api_key")

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

	decoderConfig := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   &cfg,
		TagName:  "yaml", // Explicitly use "yaml" tag for mapstructure
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(), // For time.Duration handling
			mapstructure.StringToSliceHookFunc(","),     // For slice values
		),
		WeaklyTypedInput: true, // Allow for more flexible type conversion
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create mapstructure decoder: %w", err)
	}

	if err := decoder.Decode(v.AllSettings()); err != nil {
		return nil, fmt.Errorf("failed to decode viper settings into config struct: %w", err)
	}

	// Manual unmarshalling for llm.providers
	// Clear any providers that may have been partially loaded by mapstructure
	cfg.LLM.Providers = []LLMProviderConfig{}
	if v.IsSet("llm.providers") {
		providers := v.Get("llm.providers").([]interface{})
		for _, provider := range providers {
			p := provider.(map[string]interface{})
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

	// Manually populate SecureString fields after unmarshaling
	// This is necessary because mapstructure doesn't automatically handle custom types
	// and SecureStringHookFunc was removed.
	// This assumes the API keys are coming as plaintext strings from config/env and
	// are converted to SecureString here.
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

	// Handle LLM Providers array API keys manually
	for i := range cfg.LLM.Providers {
		envKey := fmt.Sprintf("llm.providers.%d.api_key", i)
		if v.IsSet(envKey) {
			cfg.LLM.Providers[i].APIKey = domain.NewSecureStringFromString(v.GetString(envKey))
		}
	}

	// Handle Skills map API keys manually
	// This assumes skill names are already present in cfg.Skills.Entries from file or defaults
	// If skills can be defined purely by env vars, this logic needs to be more robust.
	for skillName := range cfg.Skills.Entries { // Iterate through existing entries
		envKey := fmt.Sprintf("skills.entries.%s.api_key", skillName)
		if v.IsSet(envKey) {
			s := cfg.Skills.Entries[skillName]
			s.APIKey = domain.NewSecureStringFromString(v.GetString(envKey))
			cfg.Skills.Entries[skillName] = s
		}
	}

	return &cfg, nil
}
