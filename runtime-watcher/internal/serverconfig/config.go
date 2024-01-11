package serverconfig

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
)

const (
	minPort, maxPort   = 1, 65535
	defaultWebhookPort = 8443
	defaultMetricsPort = 2112
	envWebhookPort     = "WEBHOOK_PORT"
	envMetricsPort     = "METRICS_PORT"
	envCACert          = "CA_CERT"
	envTLSCert         = "TLS_CERT"
	envTLSKey          = "TLS_KEY"
	envKCPAddress      = "KCP_ADDR"
	envKCPContract     = "KCP_CONTRACT"
)

type ServerConfig struct {
	Port        int
	MetricsPort int
	CACertPath  string
	TLSCertPath string
	TLSKeyPath  string
	KCPAddress  string
	KCPContract string
}

func ParseFromEnv(logger logr.Logger) (ServerConfig, error) {
	config := ServerConfig{}

	config.Port = defaultWebhookPort
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

	config.MetricsPort = defaultMetricsPort
	metricsPort, found := os.LookupEnv(envMetricsPort)
	if found {
		port, err := strconv.Atoi(metricsPort)
		if err != nil {
			logger.Error(err, flagError(envMetricsPort).Error())
		}
		if err = validatePortRange(port); err != nil {
			logger.Error(err, flagError(envMetricsPort).Error())
		} else {
			config.MetricsPort = port
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

func (s *ServerConfig) PrettyPrint() string {
	configValues := []string{
		fmt.Sprintf("%s: %d", envWebhookPort, s.Port),
		fmt.Sprintf("%s: %d", envMetricsPort, s.MetricsPort),
		fmt.Sprintf("%s: %s", envCACert, s.CACertPath),
		fmt.Sprintf("%s: %s", envTLSCert, s.TLSCertPath),
		fmt.Sprintf("%s: %s", envTLSKey, s.TLSKeyPath),
		fmt.Sprintf("%s: %s", envKCPAddress, s.KCPAddress),
		fmt.Sprintf("%s: %s", envKCPContract, s.KCPContract),
	}
	return strings.Join(configValues, "\n")
}
