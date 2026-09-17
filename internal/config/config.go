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
	"os"
	"path/filepath"
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

// Path is where settings are written, whether or not a file exists yet.
func (c *Config) Path() (string, error) {
	if used := c.v.ConfigFileUsed(); used != "" {
		return used, nil
	}
	home, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "lucky", "config.yaml"), nil
}

// SetVault persists which customer Lucky is working for and returns the file
// it wrote.
//
// It rewrites only what a file already held, plus the new value. The live
// viper knows more than that — flags, environment, built-in defaults — and
// writing those out would quietly turn this invocation's `--account` or a
// CI job's LUCKY_BACKEND into permanent settings on the operator's disk. A
// config file should contain what somebody chose to put in it.
func (c *Config) SetVault(name string) (string, error) {
	path, err := c.Path()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	out := viper.New()
	out.SetConfigFile(path)
	if err := out.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok && !os.IsNotExist(err) {
			return "", err
		}
	}
	if strings.TrimSpace(name) == "" {
		// Viper cannot unset a key, so the file is rebuilt without it.
		rebuilt := viper.New()
		rebuilt.SetConfigFile(path)
		for key, value := range out.AllSettings() {
			if key != KeyVault {
				rebuilt.Set(key, value)
			}
		}
		out = rebuilt
	} else {
		out.Set(KeyVault, name)
	}
	if err := out.WriteConfigAs(path); err != nil {
		return "", err
	}
	// Keep the running process consistent with what was just written.
	c.v.Set(KeyVault, name)
	return path, nil
}
