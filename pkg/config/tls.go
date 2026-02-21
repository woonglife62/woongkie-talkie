package config

import (
	"fmt"

	"github.com/jinzhu/configor"
)

type tlsConfig struct {
	CertFile string `env:"TLS_CERT_FILE"`
	KeyFile  string `env:"TLS_KEY_FILE"`
}

var TLSConfig = tlsConfig{}

// loadTLS loads TLS configuration. Called from LoadAll().
func loadTLS() error {
	if err := configor.Load(&TLSConfig); err != nil {
		return fmt.Errorf("tls configor load: %w", err)
	}
	return nil
}

// init is intentionally left empty. TLS configuration is loaded exclusively
// via loadTLS() called from LoadAll(), avoiding double-initialization (#99).
func init() {}
