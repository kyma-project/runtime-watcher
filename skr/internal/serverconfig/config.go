package serverconfig

import (
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	"os"
	"strconv"
)

const (
	minPort, maxPort      = 1, 65535
	defaultPort           = "8443"
	defaultTLSEnabledMode = false
	envWebhookPort        = "WEBHOOK_PORT"
	envTLSServer          = "TLS_SERVER"
	envTLSCallback        = "TLS_CALLBACK"
	envCACert             = "CA_CERT"
	envTLSCert            = "TLS_CERT"
	envTLSKey             = "TLS_KEY"
)

type ServerConfig struct {
	Port               string // Webhook server port
	CACert             string // CA key used to sign the certificate
	TLSCert            string // Path to TLS certificate for https
	TLSKey             string // Path to TLS key matching for certificate
	TLSEnabled         bool   // Indicates if HTTPS server should be created
	TLSCallbackEnabled bool   // Indicates if KCP accepts HTTPS requests
}

func ParseFromEnv(logger logr.Logger) (ServerConfig, error) {
	logger = logger.V(1)

	var err error
	config := ServerConfig{}

	config.Port = defaultPort
	webhookPort, found := os.LookupEnv(envWebhookPort)
	if found {
		if err = validatePortFormat(webhookPort); err != nil {
			logger.Error(err, flagError(envWebhookPort).Error())
		} else {
			config.Port = webhookPort
		}
	}

	config.TLSEnabled = defaultTLSEnabledMode
	tlsEnabled, found := os.LookupEnv(envTLSServer)
	if found {
		tlsEnabledValue, err := strconv.ParseBool(tlsEnabled)
		if err != nil {
			logger.Error(err, flagError(envTLSServer).Error())
		} else {
			config.TLSEnabled = tlsEnabledValue
		}
	}

	config.TLSCallbackEnabled = defaultTLSEnabledMode
	tlsCallbackEnabled, found := os.LookupEnv(envTLSCallback)
	if found {
		tlsCallbackEnabledValue, err := strconv.ParseBool(tlsCallbackEnabled)
		if err != nil {
			logger.Error(err, flagError(envTLSCallback).Error())
		} else {
			config.TLSCallbackEnabled = tlsCallbackEnabledValue
		}
	}

	if config.TLSEnabled || config.TLSCallbackEnabled {
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
	}
	return config, nil
}

func validatePortFormat(port string) error {
	converted, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("port is not a number: %s", err)
	}
	if converted <= minPort || converted >= maxPort {
		return errors.New("invalid port range")
	}

	return nil
}

func flagError(flagName string) error {
	return fmt.Errorf("failed parsing %s env variable", flagName)
}
