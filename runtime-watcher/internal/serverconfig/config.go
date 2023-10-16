package serverconfig

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/go-logr/logr"
)

const (
	minPort, maxPort = 1, 65535
	defaultPort      = 8443
	envWebhookPort   = "WEBHOOK_PORT"
	envCACert        = "CA_CERT"
	envTLSCert       = "TLS_CERT"
	envTLSKey        = "TLS_KEY"
	envKCPAddress    = "KCP_ADDR"
	envKCPContract   = "KCP_CONTRACT"
)

type ServerConfig struct {
	Port        int
	CACertPath  string
	TLSCertPath string
	TLSKeyPath  string
	KCPAddress  string
	KCPContract string
}

func ParseFromEnv(logger logr.Logger) (ServerConfig, error) {
	logger = logger.V(1)

	config := ServerConfig{}

	config.Port = defaultPort
	webhookPort, found := os.LookupEnv(envWebhookPort)
	if found {
		port, err := strconv.Atoi(webhookPort)
		if err != nil {
			logger.Error(err, flagError(envWebhookPort).Error())
		}
		if err = validatePortRange(port); err != nil {
			logger.Error(err, flagError(envWebhookPort).Error())
		} else {
			config.Port = port
		}
	}

	config.CACertPath = os.Getenv(envCACert)
	if config.CACertPath == "" {
		return config, flagError(envCACert)
	}
	config.TLSCertPath = os.Getenv(envTLSCert)
	if config.TLSCertPath == "" {
		return config, flagError(envTLSCert)
	}
	config.TLSKeyPath = os.Getenv(envTLSKey)
	if config.TLSKeyPath == "" {
		return config, flagError(envTLSKey)
	}
	config.KCPAddress = os.Getenv(envKCPAddress)
	if config.KCPAddress == "" {
		return config, flagError(envKCPAddress)
	}
	config.KCPContract = os.Getenv(envKCPContract)
	if config.KCPContract == "" {
		return config, flagError(envKCPContract)
	}

	return config, nil
}

func validatePortRange(port int) error {
	if port <= minPort || port >= maxPort {
		return errors.New("invalid port range")
	}

	return nil
}

func flagError(flagName string) error {
	return fmt.Errorf("failed parsing %s env variable", flagName)
}
