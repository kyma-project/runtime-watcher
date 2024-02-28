//nolint:paralleltest
package serverconfig_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyma-project/runtime-watcher/skr/internal/serverconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ParseFromEnv_PortUnsetShouldUseDefaultValue(t *testing.T) {
	setTestDefaults(t)
	logger := logr.FromContextOrDiscard(context.TODO())

	result, err := serverconfig.ParseFromEnv(logger)

	require.NoError(t, err)
	assert.Equal(t, 8443, result.Port)
}

func Test_ParseFromEnv_InvalidPortShouldUseDefaultValue(t *testing.T) {
	setTestDefaults(t)
	t.Setenv("WEBHOOK_PORT", "invalid value")
	logger := logr.FromContextOrDiscard(context.TODO())

	result, err := serverconfig.ParseFromEnv(logger)

	require.NoError(t, err)
	assert.Equal(t, 8443, result.Port)
}

func Test_ParseFromEnv_InvalidPortRangeShouldUseDefaultValue(t *testing.T) {
	setTestDefaults(t)
	t.Setenv("WEBHOOK_PORT", "65536")
	logger := logr.FromContextOrDiscard(context.TODO())

	result, err := serverconfig.ParseFromEnv(logger)

	require.NoError(t, err)
	assert.Equal(t, 8443, result.Port)
}

func Test_ParseFromEnv_ValidPort(t *testing.T) {
	setTestDefaults(t)
	t.Setenv("WEBHOOK_PORT", "1234")
	logger := logr.FromContextOrDiscard(context.TODO())

	result, err := serverconfig.ParseFromEnv(logger)

	require.NoError(t, err)
	assert.Equal(t, 1234, result.Port)
}

func Test_ParseFromEnv_CACertUnsetShouldReturnError(t *testing.T) {
	t.Setenv("TLS_CERT", "tmp")
	t.Setenv("TLS_KEY", "tmp")
	t.Setenv("KCP_ADDR", "address")
	t.Setenv("KCP_CONTRACT", "contract")
	logger := logr.FromContextOrDiscard(context.TODO())

	_, err := serverconfig.ParseFromEnv(logger)

	assert.Error(t, err)
}

func Test_ParseFromEnv_TLSCertUnsetShouldReturnError(t *testing.T) {
	t.Setenv("CA_CERT", "tmp")
	t.Setenv("TLS_KEY", "tmp")
	t.Setenv("KCP_ADDR", "address")
	t.Setenv("KCP_CONTRACT", "contract")
	logger := logr.FromContextOrDiscard(context.TODO())

	_, err := serverconfig.ParseFromEnv(logger)

	assert.Error(t, err)
}

func Test_ParseFromEnv_TLSKeyUnsetShouldReturnError(t *testing.T) {
	t.Setenv("CA_CERT", "tmp")
	t.Setenv("TLS_CERT", "tmp")
	t.Setenv("KCP_ADDR", "address")
	t.Setenv("KCP_CONTRACT", "contract")
	logger := logr.FromContextOrDiscard(context.TODO())

	_, err := serverconfig.ParseFromEnv(logger)

	assert.Error(t, err)
}

func Test_ParseFromEnv_KCPAddressUnsetShouldReturnError(t *testing.T) {
	t.Setenv("CA_CERT", "tmp")
	t.Setenv("TLS_CERT", "tmp")
	t.Setenv("TLS_KEY", "tmp")
	t.Setenv("KCP_CONTRACT", "contract")
	logger := logr.FromContextOrDiscard(context.TODO())

	_, err := serverconfig.ParseFromEnv(logger)

	assert.Error(t, err)
}

func Test_ParseFromEnv_KCPContractUnsetShouldReturnError(t *testing.T) {
	t.Setenv("CA_CERT", "tmp")
	t.Setenv("TLS_CERT", "tmp")
	t.Setenv("TLS_KEY", "tmp")
	t.Setenv("KCP_ADDR", "address")
	logger := logr.FromContextOrDiscard(context.TODO())

	_, err := serverconfig.ParseFromEnv(logger)

	assert.Error(t, err)
}

func Test_ParseFromEnv_ValidConfig(t *testing.T) {
	t.Setenv("WEBHOOK_PORT", "1234")
	t.Setenv("CA_CERT", "ca_cert_path")
	t.Setenv("TLS_CERT", "tls_cert_path")
	t.Setenv("TLS_KEY", "tls_key_path")
	t.Setenv("KCP_ADDR", "kcp_address")
	t.Setenv("KCP_CONTRACT", "kcp_contract")
	logger := logr.FromContextOrDiscard(context.TODO())

	result, err := serverconfig.ParseFromEnv(logger)

	require.NoError(t, err)
	assert.Equal(t, 1234, result.Port)
	assert.Equal(t, "ca_cert_path", result.CACertPath)
	assert.Equal(t, "tls_cert_path", result.TLSCertPath)
	assert.Equal(t, "tls_key_path", result.TLSKeyPath)
	assert.Equal(t, "kcp_address", result.KCPAddress)
	assert.Equal(t, "kcp_contract", result.KCPContract)
}

func setTestDefaults(t *testing.T) {
	t.Helper()
	t.Setenv("CA_CERT", "tmp")
	t.Setenv("TLS_CERT", "tmp")
	t.Setenv("TLS_KEY", "tmp")
	t.Setenv("KCP_ADDR", "address")
	t.Setenv("KCP_CONTRACT", "contract")
}
