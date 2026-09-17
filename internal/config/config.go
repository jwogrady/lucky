// Package config resolves Lucky's settings in one place.
//
// Cobra and viper between them already provide flag/env/file precedence,
// config discovery, and typed access. Most of that infrastructure is invisible
// when it works, which is the argument for using it rather than reading
// os.Getenv in four files and inventing a precedence order by accident.
//
// Precedence, highest first:
//
//	--flag            explicit, this invocation
//	LUCKY_* env var   the shell, CI, a service account context
//	config file       ~/.config/lucky/config.yaml
//	built-in default
//
// Nothing secret belongs in the config file, and nothing here reads one. The
// file holds which account, which vault, which backend — never a credential.
// OP_SERVICE_ACCOUNT_TOKEN is read from the environment only, deliberately, so
// it cannot come to rest on disk through this path.
package config

import (
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	KeyAccount   = "account"
	KeyVault     = "vault"
	KeyTemplates = "templates"
	KeyBackend   = "backend"
	KeyOpBin     = "op-bin"
)

type Config struct{ v *viper.Viper }

// New builds the resolver. Call Bind for each command's flags.
func New() *Config {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("$HOME/.config/lucky")
	v.AddConfigPath(".")

	v.SetEnvPrefix("LUCKY")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	v.SetDefault(KeyBackend, "auto")
	v.SetDefault(KeyTemplates, "")

	// A missing config file is the normal case, not an error.
	_ = v.ReadInConfig()
	return &Config{v: v}
}

// Bind attaches a command's flags so an explicit flag outranks the environment.
func (c *Config) Bind(flags *pflag.FlagSet) {
	for _, key := range []string{KeyAccount, KeyVault, KeyTemplates, KeyBackend, KeyOpBin} {
		if f := flags.Lookup(key); f != nil {
			_ = c.v.BindPFlag(key, f)
		}
	}
}

func (c *Config) String(key string) string { return strings.TrimSpace(c.v.GetString(key)) }

func (c *Config) Account() string   { return c.String(KeyAccount) }
func (c *Config) Vault() string     { return c.String(KeyVault) }
func (c *Config) Templates() string { return c.String(KeyTemplates) }
func (c *Config) Backend() string   { return c.String(KeyBackend) }
func (c *Config) OpBin() string     { return c.String(KeyOpBin) }

// Used reports which config file was loaded, for `lucky status` to show. Empty
// means none was found, which is fine.
func (c *Config) Used() string { return c.v.ConfigFileUsed() }
