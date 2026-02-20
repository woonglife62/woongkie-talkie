package config

import "github.com/jinzhu/configor"

type tlsConfig struct {
	CertFile string `env:"TLS_CERT_FILE"`
	KeyFile  string `env:"TLS_KEY_FILE"`
}

var TLSConfig = tlsConfig{}

func init() {
	configor.Load(&TLSConfig)
}
