package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/jon4hz/jellysweep/internal/logging"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var v = viper.New()

// MustBindPFlag binds a cobra persistent flag to a viper key.
func MustBindPFlag(key string, flag *pflag.Flag) {
	if err := v.BindPFlag(key, flag); err != nil {
		panic(err)
	}
}

const defaultTimeout = 30 // seconds

// TimeoutDuration returns the configured timeout as a time.Duration.
// If the timeout is 0, it returns the default of 30 seconds.
func TimeoutDuration(seconds int) time.Duration {
	if seconds <= 0 {
		return time.Duration(defaultTimeout) * time.Second
	}
	return time.Duration(seconds) * time.Second
}

type CacheType string

const (
	CacheTypeMemory CacheType = "memory"
	CacheTypeRedis  CacheType = "redis"
)

type CleanupMode string

const (
	CleanupModeAll          CleanupMode = "all"
	CleanupModeKeepEpisodes CleanupMode = "keep_episodes"
	CleanupModeKeepSeasons  CleanupMode = "keep_seasons"
)

// Config holds the configuration for the Jellysweep server and its dependencies.
type Config struct {
	// LogLevel sets the log verbosity. Options: "debug", "info", "warn", "error". Defaults to "info".
	LogLevel string `yaml:"log_level" mapstructure:"log_level"`
	// Listen is the address the Jellysweep server will listen on.
	Listen string `yaml:"listen" mapstructure:"listen"`
	// CleanupSchedule is the cron schedule for the cleanup job (e.g., "0 */12 * * *" for every 12 hours).
	CleanupSchedule string `yaml:"cleanup_schedule" mapstructure:"cleanup_schedule"`
	// Libraries is a map of libraries to their cleanup configurations.
	Libraries map[string]*CleanupConfig `yaml:"libraries" mapstructure:"libraries"`
	// DryRun indicates whether the cleanup job should run in dry-run mode.
	DryRun bool `yaml:"dry_run" mapstructure:"dry_run"`
	// CleanupMode specifies how to clean up TV series. Options: "all", "keep_episodes", "keep_seasons"
	// See engine.CleanupMode* constants for valid values.
	CleanupMode CleanupMode `yaml:"cleanup_mode" mapstructure:"cleanup_mode"`
	// KeepCount specifies how many episodes or seasons to keep when using "keep_episodes" or "keep_seasons" mode
	KeepCount int `yaml:"keep_count" mapstructure:"keep_count"`
	// Auth holds the authentication configuration for the Jellysweep server.
	Auth *AuthConfig `yaml:"auth" mapstructure:"auth"`
	// Database holds the database configuration.
	Database *DatabaseConfig `yaml:"database" mapstructure:"database"`
	// APIKey is the API key for the Jellysweep server (used by the jellyfin plugin).
	APIKey string `yaml:"api_key" mapstructure:"api_key"`
	// SessionKey is the key used to encrypt session data.
	SessionKey string `yaml:"session_key" mapstructure:"session_key"`
	// SessionMaxAge is the maximum age of a session in seconds.
	SessionMaxAge int `yaml:"session_max_age" mapstructure:"session_max_age"`
	// SecureCookies sets the Secure flag on session cookies. Defaults to true.
	// Set to false only for local development without TLS.
	SecureCookies bool `yaml:"secure_cookies" mapstructure:"secure_cookies"`
	// TrustedProxies is a list of trusted proxy IP addresses or CIDR ranges.
	// Set to null to trust all proxies.
	TrustedProxies []string `yaml:"trusted_proxies" mapstructure:"trusted_proxies"`
	// Email holds the email notification configuration.
	Email *EmailConfig `yaml:"email" mapstructure:"email"`
	// Ntfy holds the ntfy notification configuration.
	Ntfy *NtfyConfig `yaml:"ntfy" mapstructure:"ntfy"`
	// WebPush holds the webpush notification configuration.
	WebPush *WebPushConfig `yaml:"webpush" mapstructure:"webpush"`
	// ServerURL is the base URL of the Jellysweep server.
	ServerURL string `yaml:"server_url" mapstructure:"server_url"`
	// Cache holds the cache engine configuration.
	Cache *CacheConfig `yaml:"cache" mapstructure:"cache"`
	// LeavingCollectionsEnabled controls whether "Leaving Soon" collections are created in Jellyfin.
	LeavingCollectionsEnabled bool `yaml:"leaving_collections_enabled" mapstructure:"leaving_collections_enabled"`
	// Name of the "Leaving Movies" collection in Jellyfin.
	LeavingCollectionsMovieName string `yaml:"leaving_collections_movie_name" mapstructure:"leaving_collections_movie_name"`
	// Name of the "Leaving TV Shows" collection in Jellyfin.
	LeavingCollectionsTVName string `yaml:"leaving_collections_tv_name" mapstructure:"leaving_collections_tv_name"`

	// Jellyseerr holds the configuration for the Jellyseerr server.
	Jellyseerr *JellyseerrConfig `yaml:"jellyseerr" mapstructure:"jellyseerr"`
	// Sonarr holds the configuration for the Sonarr server.
	Sonarr *SonarrConfig `yaml:"sonarr" mapstructure:"sonarr"`
	// Radarr holds the configuration for the Radarr server.
	Radarr *RadarrConfig `yaml:"radarr" mapstructure:"radarr"`
	// Jellystat holds the configuration for the Jellystat server.
	Jellystat *JellystatConfig `yaml:"jellystat" mapstructure:"jellystat"`
	// Gravatar holds the configuration for Gravatar profile pictures.
	Gravatar *GravatarConfig `yaml:"gravatar" mapstructure:"gravatar"`
	// Jellyfin holds the configuration for the Jellyfin server.
	Jellyfin *JellyfinConfig `yaml:"jellyfin" mapstructure:"jellyfin"`
	// Tunarr holds the configuration for the Tunarr server.
	Tunarr *TunarrConfig `yaml:"tunarr" mapstructure:"tunarr"`
}

// AuthConfig holds the authentication configuration for the Jellysweep server.
type AuthConfig struct {
	// OIDC holds the OpenID Connect configuration.
	OIDC *OIDCConfig `yaml:"oidc" mapstructure:"oidc"`
	// Jellyfin holds the Jellyfin authentication configuration.
	Jellyfin *JellyfinAuthConfig `yaml:"jellyfin" mapstructure:"jellyfin"`
}

// OIDCConfig holds the OpenID Connect configuration for the Jellysweep server.
type OIDCConfig struct {
	// Enabled indicates whether OIDC authentication is enabled.
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	// Name is the display name for the OIDC provider.
	Name string `yaml:"name" mapstructure:"name"`
	// Issuer is the OIDC issuer URL.
	Issuer string `yaml:"issuer" mapstructure:"issuer"`
	// ClientID is the OIDC client ID.
	ClientID string `yaml:"client_id" mapstructure:"client_id"`
	// ClientSecret is the OIDC client secret.
	ClientSecret string `yaml:"client_secret" mapstructure:"client_secret"`
	// RedirectURL is the redirect URL for the oidc flow.
	RedirectURL string `yaml:"redirect_url" mapstructure:"redirect_url"`
	// AdminGroup is the group that has admin privileges.
	AdminGroup string `yaml:"admin_group" mapstructure:"admin_group"`
	// AutoApproveGroup is the group that gets automatic approval for keep requests.
	// Members of this group will have their keep requests automatically approved without admin intervention.
	// This setting overrides the database value for auto-approval permission on each login.
	AutoApproveGroup string `yaml:"auto_approve_group" mapstructure:"auto_approve_group"`
	// UsePKCE enables PKCE (Proof Key for Code Exchange) for the OAuth 2.0 flow.
	UsePKCE bool `yaml:"use_pkce" mapstructure:"use_pkce"`
	// Timeout is the HTTP client timeout in seconds for OIDC provider requests.
	Timeout int `yaml:"timeout" mapstructure:"timeout"`
}

// JellyfinAuthConfig holds the Jellyfin authentication configuration for the Jellysweep server.
type JellyfinAuthConfig struct {
	// Enabled indicates whether Jellyfin authentication is enabled.
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
}

// DatabaseConfig holds the database configuration.
type DatabaseConfig struct {
	// Path is the path to the database file.
	Path string `yaml:"path" mapstructure:"path"`
}

// EmailConfig holds the email notification configuration.
type EmailConfig struct {
	// Enabled indicates whether email notifications are enabled.
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	// SMTPHost is the SMTP server host.
	SMTPHost string `yaml:"smtp_host" mapstructure:"smtp_host"`
	// SMTPPort is the SMTP server port.
	SMTPPort int `yaml:"smtp_port" mapstructure:"smtp_port"`
	// Username is the SMTP username.
	Username string `yaml:"username" mapstructure:"username"`
	// Password is the SMTP password.
	Password string `yaml:"password" mapstructure:"password"`
	// FromEmail is the email address from which notifications are sent.
	FromEmail string `yaml:"from_email" mapstructure:"from_email"`
	// FromName is the name from which notifications are sent.
	FromName string `yaml:"from_name" mapstructure:"from_name"`
	// UseTLS indicates whether to use TLS for the SMTP connection.
	UseTLS bool `yaml:"use_tls" mapstructure:"use_tls"`
	// UseSSL indicates whether to use SSL for the SMTP connection.
	UseSSL bool `yaml:"use_ssl" mapstructure:"use_ssl"`
	// InsecureSkipVerify indicates whether to skip TLS certificate verification.
	InsecureSkipVerify bool `yaml:"insecure_skip_verify" mapstructure:"insecure_skip_verify"`
}

// NtfyConfig holds the ntfy notification configuration.
type NtfyConfig struct {
	// Enabled indicates whether ntfy notifications are enabled.
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	// ServerURL is the URL of the ntfy server.
	ServerURL string `yaml:"server_url" mapstructure:"server_url"`
	// Topic is the ntfy topic to publish notifications to.
	Topic string `yaml:"topic" mapstructure:"topic"`
	// Username is the ntfy username for authentication.
	Username string `yaml:"username" mapstructure:"username"`
	// Password is the ntfy password for authentication.
	Password string `yaml:"password" mapstructure:"password"`
	// Token is the ntfy token for authentication.
	Token string `yaml:"token" mapstructure:"token"`
	// Timeout is the HTTP client timeout in seconds.
	Timeout int `yaml:"timeout" mapstructure:"timeout"`
}

// WebPushConfig holds the webpush notification configuration.
type WebPushConfig struct {
	// Enabled indicates whether webpush notifications are enabled.
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	// VAPIDEmail is the email associated with the VAPID keys.
	VAPIDEmail string `yaml:"vapid_email" mapstructure:"vapid_email"`
	// PublicKey is the VAPID public key.
	PublicKey string `yaml:"public_key" mapstructure:"public_key"`
	// PrivateKey is the VAPID private key.
	PrivateKey string `yaml:"private_key" mapstructure:"private_key"`
	// Timeout is the HTTP client timeout in seconds.
	Timeout int `yaml:"timeout" mapstructure:"timeout"`
}

// CleanupConfig is the YAML shape used to seed a library's settings into the database on first run.
// After the first run the database is authoritative; further YAML changes are ignored.
type CleanupConfig struct {
	// Enabled controls whether this library is swept at all.
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	// LifetimeDays is the number of days from import-date before a movie/show is auto-queued for deletion.
	LifetimeDays int `yaml:"lifetime_days" mapstructure:"lifetime_days"`
	// DeletionPeriodDays is the grace period (in days) between queueing and actual deletion.
	DeletionPeriodDays int `yaml:"deletion_period_days" mapstructure:"deletion_period_days"`
	// ProtectionDays is the number of days an item stays protected after a keep request is approved.
	ProtectionDays int `yaml:"protection_days" mapstructure:"protection_days"`
	// CompletionThresholdPct is the playback-progress percent that counts as "watched" for movies (1-100).
	CompletionThresholdPct int `yaml:"completion_threshold_pct" mapstructure:"completion_threshold_pct"`
}

// CacheConfig holds the configuration for the cache engine.
type CacheConfig struct {
	// Type is the type of cache engine to use (e.g., "memory", "redis").
	Type CacheType `yaml:"type" mapstructure:"type"`
	// RedisURL is the URL for the Redis cache if using Redis.
	RedisURL string `yaml:"redis_url" mapstructure:"redis_url"`
}

// JellyseerrConfig holds the configuration for the Jellyseerr server.
type JellyseerrConfig struct {
	// URL is the base URL of the Jellyseerr server.
	URL string `yaml:"url" mapstructure:"url"`
	// APIKey is the API key for the Jellyseerr server.
	APIKey string `yaml:"api_key" mapstructure:"api_key"`
	// Timeout is the HTTP client timeout in seconds.
	Timeout int `yaml:"timeout" mapstructure:"timeout"`
}

// SonarrConfig holds the configuration for the Sonarr server.
type SonarrConfig struct {
	// URL is the base URL of the Sonarr server.
	URL string `yaml:"url" mapstructure:"url"`
	// APIKey is the API key for the Sonarr server.
	APIKey string `yaml:"api_key" mapstructure:"api_key"`
	// Timeout is the HTTP client timeout in seconds.
	Timeout int `yaml:"timeout" mapstructure:"timeout"`
}

// RadarrConfig holds the configuration for the Radarr server.
type RadarrConfig struct {
	// URL is the base URL of the Radarr server.
	URL string `yaml:"url" mapstructure:"url"`
	// APIKey is the API key for the Radarr server.
	APIKey string `yaml:"api_key" mapstructure:"api_key"`
	// Timeout is the HTTP client timeout in seconds.
	Timeout int `yaml:"timeout" mapstructure:"timeout"`
}

// JellystatConfig holds the configuration for the Jellystat server.
type JellystatConfig struct {
	// URL is the base URL of the Jellystat server.
	URL string `yaml:"url" mapstructure:"url"`
	// APIKey is the API key for the Jellystat server.
	APIKey string `yaml:"api_key" mapstructure:"api_key"`
	// Timeout is the HTTP client timeout in seconds.
	Timeout int `yaml:"timeout" mapstructure:"timeout"`
}

// TunarrConfig holds the configuration for the Tunarr server.
type TunarrConfig struct {
	// URL is the base URL of the Tunarr server.
	URL string `yaml:"url" mapstructure:"url"`
	// Timeout is the HTTP client timeout in seconds.
	Timeout int `yaml:"timeout" mapstructure:"timeout"`
}

// JellyfinConfig holds the configuration for the Jellyfin server.
type JellyfinConfig struct {
	// URL is the base URL of the Jellyfin server.
	URL string `yaml:"url" mapstructure:"url"`
	// APIKey is the API key for the Jellyfin server.
	APIKey string `yaml:"api_key" mapstructure:"api_key"`
	// Timeout is the HTTP client timeout in seconds.
	Timeout int `yaml:"timeout" mapstructure:"timeout"`
}

// GravatarConfig holds the configuration for Gravatar profile pictures.
type GravatarConfig struct {
	// Enabled indicates whether Gravatar support is enabled.
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	// DefaultImage is the default image to use when no Gravatar is found.
	// Valid values: "404", "mp", "identicon", "monsterid", "wavatar", "retro", "robohash", "blank"
	DefaultImage string `yaml:"default_image" mapstructure:"default_image"`
	// Rating is the maximum rating for Gravatar images.
	// Valid values: "g", "pg", "r", "x"
	Rating string `yaml:"rating" mapstructure:"rating"`
	// Size is the size of the Gravatar image in pixels (1-2048).
	Size int `yaml:"size" mapstructure:"size"`
}

// Load reads the configuration from the specified path and returns a Config struct.
// If path is empty, it will use default search paths for config files.
// If no config file is found, it will generate a default one in the current directory.
func Load(path string) (*Config, error) {
	// bind some weirdly unsupported nested env vars
	bindNestedEnv(v)

	// Set default values
	setDefaults(v)

	// Configure Viper
	v.SetConfigType("yaml")
	v.SetEnvPrefix("JELLYSWEEP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var configFileFound bool
	if path != "" {
		// Use specific config file
		v.SetConfigFile(path)
	} else {
		// Search for config in common locations
		v.SetConfigName("config")
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.jellysweep")
		v.AddConfigPath("/etc/jellysweep")
	}

	// Read the config file
	if err := v.ReadInConfig(); err != nil {
		// If no config file is found, use defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	} else {
		configFileFound = true
	}

	// Print info about config file usage
	if configFileFound {
		log.Debug("Using config file", "file", v.ConfigFileUsed())
		log.Debug("Some environment variables can be set with the JELLYSWEEP_ prefix to override config file values")
	}

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Apply the resolved log level.
	logging.SetLevel(c.LogLevel)

	// Sanitize config values
	sanitizeConfig(&c)

	// Validate required configs
	if err := validateConfig(&c); err != nil {
		return nil, err
	}

	return &c, nil
}

// setDefaults sets default values for the configuration.
func setDefaults(v *viper.Viper) {
	// Jellysweep defaults
	v.SetDefault("log_level", "info")
	v.SetDefault("listen", "0.0.0.0:3002")
	v.SetDefault("cleanup_schedule", "0 */12 * * *") // Every 12 hours
	v.SetDefault("cleanup_mode", "all")              // Default to cleaning up everything
	v.SetDefault("keep_count", 1)                    // Default to keeping 1 episode/season if mode is not "all"
	v.SetDefault("dry_run", true)
	v.SetDefault("server_url", "http://localhost:3002")
	v.SetDefault("session_max_age", 172800) // 48 hour
	v.SetDefault("session_key", "")
	v.SetDefault("secure_cookies", true)
	v.SetDefault("api_key", "")

	// Auth defaults
	v.SetDefault("auth.oidc.enabled", false)
	v.SetDefault("auth.oidc.name", "OIDC")
	v.SetDefault("auth.oidc.issuer", "")
	v.SetDefault("auth.oidc.client_id", "")
	v.SetDefault("auth.oidc.client_secret", "")
	v.SetDefault("auth.oidc.redirect_url", "")
	v.SetDefault("auth.oidc.use_pkce", false)
	v.SetDefault("auth.oidc.admin_group", "")
	v.SetDefault("auth.oidc.auto_approve_group", "")
	v.SetDefault("auth.oidc.timeout", 30)
	v.SetDefault("auth.jellyfin.enabled", true)

	// Database defaults
	v.SetDefault("database.path", "./data/jellysweep.db")

	// Cache defaults
	v.SetDefault("cache.type", CacheTypeMemory) // Default to in-memory
	v.SetDefault("cache.redis_url", "")

	// Leaving collections default
	v.SetDefault("enable_leaving_collections", false)
	v.SetDefault("leaving_collections_movie_name", "Leaving Movies")
	v.SetDefault("leaving_collections_tv_name", "Leaving TV Shows")

	// Email defaults
	v.SetDefault("email.enabled", false)
	v.SetDefault("email.smtp_host", "")
	v.SetDefault("email.smtp_port", 587)
	v.SetDefault("email.username", "")
	v.SetDefault("email.password", "")
	v.SetDefault("email.from_name", "Jellysweep")
	v.SetDefault("email.use_tls", true)
	v.SetDefault("email.use_ssl", false)
	v.SetDefault("email.insecure_skip_verify", false)

	// Ntfy defaults
	v.SetDefault("ntfy.enabled", false)
	v.SetDefault("ntfy.server_url", "https://ntfy.sh")
	v.SetDefault("ntfy.topic", "jellysweep")
	v.SetDefault("ntfy.username", "")
	v.SetDefault("ntfy.password", "")
	v.SetDefault("ntfy.token", "")
	v.SetDefault("ntfy.timeout", 30)

	// Gravatar defaults
	v.SetDefault("gravatar.enabled", false)
	v.SetDefault("gravatar.default_image", "robohash")
	v.SetDefault("gravatar.rating", "g")
	v.SetDefault("gravatar.size", 80)

	// WebPush defaults
	v.SetDefault("webpush.enabled", false)
	v.SetDefault("webpush.vapid_email", "")
	v.SetDefault("webpush.public_key", "")
	v.SetDefault("webpush.private_key", "")
	v.SetDefault("webpush.timeout", 30)
}

// the auto env function from viper only works for nested structs, if the struct to which a value binds isn't nil.
// If we explicitly don't want a default value (e.g. because a struct value should be nil on purpose)
// we have to bind the env var manually.
func bindNestedEnv(v *viper.Viper) {
	// Jellyseerr
	v.MustBindEnv("jellyseerr.url", "JELLYSWEEP_JELLYSEERR_URL")
	v.MustBindEnv("jellyseerr.api_key", "JELLYSWEEP_JELLYSEERR_API_KEY")
	v.MustBindEnv("jellyseerr.timeout", "JELLYSWEEP_JELLYSEERR_TIMEOUT")

	// Sonarr
	v.MustBindEnv("sonarr.url", "JELLYSWEEP_SONARR_URL")
	v.MustBindEnv("sonarr.api_key", "JELLYSWEEP_SONARR_API_KEY")
	v.MustBindEnv("sonarr.timeout", "JELLYSWEEP_SONARR_TIMEOUT")

	// Radarr
	v.MustBindEnv("radarr.url", "JELLYSWEEP_RADARR_URL")
	v.MustBindEnv("radarr.api_key", "JELLYSWEEP_RADARR_API_KEY")
	v.MustBindEnv("radarr.timeout", "JELLYSWEEP_RADARR_TIMEOUT")

	// Jellystat
	v.MustBindEnv("jellystat.url", "JELLYSWEEP_JELLYSTAT_URL")
	v.MustBindEnv("jellystat.api_key", "JELLYSWEEP_JELLYSTAT_API_KEY")
	v.MustBindEnv("jellystat.timeout", "JELLYSWEEP_JELLYSTAT_TIMEOUT")

	// Tunarr
	v.MustBindEnv("tunarr.url", "JELLYSWEEP_TUNARR_URL")
	v.MustBindEnv("tunarr.timeout", "JELLYSWEEP_TUNARR_TIMEOUT")

	// Jellyfin
	v.MustBindEnv("jellyfin.url", "JELLYSWEEP_JELLYFIN_URL")
	v.MustBindEnv("jellyfin.api_key", "JELLYSWEEP_JELLYFIN_API_KEY")
	v.MustBindEnv("jellyfin.timeout", "JELLYSWEEP_JELLYFIN_TIMEOUT")
}

// validateConfig validates the configuration.
func validateConfig(c *Config) error {
	if c == nil {
		return fmt.Errorf("missing jellysweep config")
	}

	// Validate cleanup schedule
	if c.CleanupSchedule == "" {
		return fmt.Errorf("cleanup schedule is required")
	}
	// Basic validation for cron format (5 fields)
	cronFields := strings.Fields(c.CleanupSchedule)
	if len(cronFields) != 5 {
		return fmt.Errorf("cleanup schedule must be a valid cron expression with 5 fields (minute hour day month weekday)")
	}

	if c.CleanupMode == "" {
		return fmt.Errorf("cleanup mode is required")
	}

	switch c.CleanupMode {
	case CleanupModeAll, CleanupModeKeepEpisodes, CleanupModeKeepSeasons:
		// valid
	default:
		return fmt.Errorf(
			"invalid cleanup mode %q: must be one of %q, %q, %q",
			c.CleanupMode,
			CleanupModeAll,
			CleanupModeKeepEpisodes,
			CleanupModeKeepSeasons,
		)
	}

	if c.CleanupMode == CleanupModeKeepEpisodes || c.CleanupMode == CleanupModeKeepSeasons {
		if c.KeepCount <= 0 {
			return fmt.Errorf("keep count must be greater than 0 when using keep_episodes or keep_seasons mode")
		}
	}

	if c.SessionKey == "" {
		return fmt.Errorf("session key is required")
	}

	if len(c.Libraries) == 0 {
		return fmt.Errorf("at least one library must be configured")
	}

	// Validate auth configuration
	if c.Auth == nil {
		return fmt.Errorf("missing auth config")
	}

	authEnabled := false
	if c.Auth.OIDC != nil && c.Auth.OIDC.Enabled {
		authEnabled = true
		if c.Auth.OIDC.Issuer == "" {
			return fmt.Errorf("OIDC issuer is required when OIDC is enabled")
		}
		if c.Auth.OIDC.ClientID == "" {
			return fmt.Errorf("OIDC client ID is required when OIDC is enabled")
		}
		if c.Auth.OIDC.ClientSecret == "" {
			return fmt.Errorf("OIDC client secret is required when OIDC is enabled")
		}
		if c.Auth.OIDC.RedirectURL == "" {
			return fmt.Errorf("OIDC redirect URL is required when OIDC is enabled")
		}
		if c.Auth.OIDC.AdminGroup == "" {
			return fmt.Errorf("OIDC admin group is required when OIDC is enabled")
		}
	}

	if c.Cache != nil {
		if c.Cache.Type == "" {
			return fmt.Errorf("cache type is required when cache is enabled")
		}
		if c.Cache.Type == CacheTypeRedis && c.Cache.RedisURL == "" {
			return fmt.Errorf("Redis URL is required when Redis cache is enabled") //nolint:staticcheck
		}
	} else {
		c.Cache = &CacheConfig{
			Type: CacheTypeMemory, // Default to in-memory cache if not enabled
		}
	}

	if c.Jellyfin == nil {
		return fmt.Errorf("missing jellyfin config")
	}
	if c.Jellyfin.URL == "" {
		return fmt.Errorf("jellyfin URL is required")
	}
	if c.Jellyfin.APIKey == "" {
		return fmt.Errorf("jellyfin API key is required")
	}

	if c.Auth.Jellyfin != nil && c.Auth.Jellyfin.Enabled {
		authEnabled = true
		if c.Jellyfin.URL == "" {
			return fmt.Errorf("Jellyfin URL is required when Jellyfin auth is enabled") //nolint:staticcheck
		}
	}

	if !authEnabled {
		return fmt.Errorf("at least one authentication method must be enabled")
	}

	if c.Jellyseerr != nil {
		if c.Jellyseerr.URL == "" {
			return fmt.Errorf("jellyseerr URL is required")
		}
		if c.Jellyseerr.APIKey == "" {
			return fmt.Errorf("jellyseerr API key is required")
		}
	}

	if c.Sonarr == nil && c.Radarr == nil {
		return fmt.Errorf("either sonarr or radarr config must be provided")
	}

	if c.Sonarr != nil {
		if c.Sonarr.URL == "" {
			return fmt.Errorf("sonarr URL is required when sonarr is configured")
		}
		if c.Sonarr.APIKey == "" {
			return fmt.Errorf("sonarr API key is required when sonarr is configured")
		}
	}

	if c.Radarr != nil {
		if c.Radarr.URL == "" {
			return fmt.Errorf("radarr URL is required when radarr is configured")
		}
		if c.Radarr.APIKey == "" {
			return fmt.Errorf("radarr API key is required when radarr is configured")
		}
	}

	if c.Jellystat == nil {
		return fmt.Errorf("jellystat config must be provided")
	}
	if c.Jellystat.URL == "" {
		return fmt.Errorf("jellystat URL is required")
	}
	if c.Jellystat.APIKey == "" {
		return fmt.Errorf("jellystat API key is required")
	}

	if c.Tunarr != nil {
		if c.Tunarr.URL == "" {
			return fmt.Errorf("tunarr URL is required when tunarr is configured")
		}
	}

	if c.Email != nil && c.Email.Enabled {
		if c.Email.SMTPHost == "" {
			return fmt.Errorf("SMTP host is required when email notifications are enabled")
		}
		if c.Email.FromEmail == "" {
			return fmt.Errorf("from email is required when email notifications are enabled")
		}
	}

	if c.Ntfy != nil && c.Ntfy.Enabled {
		if c.Ntfy.ServerURL == "" {
			return fmt.Errorf("ntfy server URL is required when ntfy notifications are enabled")
		}
		if c.Ntfy.Topic == "" {
			return fmt.Errorf("ntfy topic is required when ntfy notifications are enabled")
		}
	}

	if c.WebPush != nil && c.WebPush.Enabled {
		if c.WebPush.PublicKey == "" || c.WebPush.PrivateKey == "" {
			return fmt.Errorf("VAPID public and private keys are required when webpush is enabled")
		}
	}

	return nil
}

// sanitizeConfig sanitizes the configuration values.
func sanitizeConfig(c *Config) {
	if c == nil {
		return
	}

	c.Listen = urlSanitize(c.Listen)

	if c.Jellyfin != nil {
		c.Jellyfin.URL = urlSanitize(c.Jellyfin.URL)
	}

	if c.Jellyseerr != nil {
		c.Jellyseerr.URL = urlSanitize(c.Jellyseerr.URL)
	}

	if c.Sonarr != nil {
		c.Sonarr.URL = urlSanitize(c.Sonarr.URL)
	}

	if c.Radarr != nil {
		c.Radarr.URL = urlSanitize(c.Radarr.URL)
	}

	if c.Jellystat != nil {
		c.Jellystat.URL = urlSanitize(c.Jellystat.URL)
	}

	if c.Tunarr != nil {
		c.Tunarr.URL = urlSanitize(c.Tunarr.URL)
	}

	if c.ServerURL != "" {
		c.ServerURL = urlSanitize(c.ServerURL)
	}
}

func urlSanitize(url string) string {
	return strings.TrimSuffix(strings.TrimSpace(url), "/")
}
