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
	envTLSCallback   = "TLS_CALLBACK"
	envCACert        = "CA_CERT"
	envTLSCert       = "TLS_CERT"
	envTLSKey        = "TLS_KEY"
	envKCPAddress    = "KCP_ADDR"
	envKCPContract   = "KCP_CONTRACT"
)

type ServerConfig struct {
	Port               int    // Webhook server port
	CACert             string // CA key used to sign the certificate
	TLSCert            string // Path to TLS certificate for https
	TLSKey             string // Path to TLS key matching for certificate
	TLSCallbackEnabled bool   // Indicates if KCP accepts HTTPS requests
	KCPAddress         string
	KCPContract        string
}

func ParseFromEnv(logger logr.Logger) (ServerConfig, error) {
	logger = logger.V(1)

	config := ServerConfig{}

	config.Port = defaultPort
	webhookPort, found := os.LookupEnv(envWebhookPort)
	if found {
		port, err := strconv.Atoi(webhookPort)
		if err = validatePortRange(port); err != nil {
			logger.Error(err, flagError(envWebhookPort).Error())
		} else {
			config.Port = port
		}
	}

	tlsCallbackEnabled, found := os.LookupEnv(envTLSCallback)
	if found {
		tlsCallbackEnabledValue, err := strconv.ParseBool(tlsCallbackEnabled)
		if err != nil {
			logger.Error(err, flagError(envTLSCallback).Error())
		} else {
			config.TLSCallbackEnabled = tlsCallbackEnabledValue
		}
	}

	config.CACert = os.Getenv(envCACert)
	if config.CACert == "" {
		return config, flagError(envCACert)
	}
	config.TLSCert = os.Getenv(envTLSCert)
	if config.TLSCert == "" {
		return config, flagError(envTLSCert)
	}
	config.TLSKey = os.Getenv(envTLSKey)
	if config.TLSKey == "" {
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
