package tests

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/castyapp/grpc.server/config"
)

var defaultConfig = &config.ConfigMap{
	Debug:    false,
	Metrics:  false,
	Env:      "dev",
	Timezone: "America/California",
	Redis: config.RedisConfig{
		Cluster:    false,
		MasterName: "casty",
		Addr:       "casty.redis:6379",
		Pass:       "super-secure-redis-password",
	},
	DB: config.DBConfig{
		Name: "casty",
		Host: "casty.db",
		Port: 27017,
		User: "gotest",
		Pass: "super-secure-mongodb-password",
	},
	JWT: config.JWTConfig{
		AccessToken: config.JWTToken{
			Secret: "random-secret",
			ExpiresAt: config.JWTExpiresAt{
				Type:  "days",
				Value: 1,
			},
		},
		RefreshToken: config.JWTToken{
			Secret: "random-secret",
			ExpiresAt: config.JWTExpiresAt{
				Type:  "weeks",
				Value: 1,
			},
		},
	},
	Oauth: config.OauthConfig{
		RegistrationByOauth: true,
		Google: config.OauthClient{
			Enabled:      false,
			ClientID:     "",
			ClientSecret: "",
			AuthUri:      "https://accounts.google.com/o/oauth2/auth",
			TokenUri:     "https://oauth2.googleapis.com/token",
			RedirectUri:  "https://casty.ir/oauth/google/callback",
		},
		Spotify: config.OauthClient{
			Enabled:      false,
			ClientID:     "",
			ClientSecret: "",
			AuthUri:      "https://accounts.spotify.com/authorize",
			TokenUri:     "https://accounts.spotify.com/api/token",
			RedirectUri:  "https://casty.ir/oauth/spotify/callback",
		},
	},
	S3: config.S3Config{
		Endpoint:  "127.0.0.1:9000",
		AccessKey: "secret-access-key",
		SecretKey: "secret-key",
	},
	Sentry: config.SentryConfig{
		Enabled: false,
		Dsn:     "sentry.dsn.here",
	},
	Recaptcha: config.RecaptchaConfig{
		Enabled: false,
		Type:    "hcaptcha",
		Secret:  "hcaptcha-secret-token",
	},
}

func TestLoadConfig(t *testing.T) {
	configMap, err := config.LoadFile(filepath.Join(configFileName))
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !reflect.DeepEqual(defaultConfig, configMap) {
		t.Fatalf("bad: %#v", configMap)
	}
}