package tls

import (
	"crypto/tls"
	"fmt"
	"os"
)

type Config struct {
	CertFile string
	KeyFile  string
	MinVersion uint16
}

func NewConfig(certFile, keyFile string) *Config {
	return &Config{
		CertFile:  certFile,
		KeyFile:   keyFile,
		MinVersion: tls.VersionTLS12,
	}
}

func (c *Config) Load() (*tls.Config, error) {
	cert, err := os.ReadFile(c.CertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate: %w", err)
	}

	key, err := os.ReadFile(c.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read key: %w", err)
	}

	certificate, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate pair: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   c.MinVersion,
		ServerName:   "",
	}

	return tlsConfig, nil
}

func (c *Config) SetMinVersion(version uint16) {
	c.MinVersion = version
}
