package serverconfig

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

func Test_ParseFromEnv_PortUnsetShouldUseDefaultValue(t *testing.T) {
	logger := logr.FromContextOrDiscard(context.TODO())

	result, err := ParseFromEnv(logger)

	assert.NoError(t, err)
	assert.Equal(t, "8443", result.Port)
}

func Test_ParseFromEnv_InvalidPortShouldUseDefaultValue(t *testing.T) {
	t.Setenv("WEBHOOK_PORT", "invalid value")
	logger := logr.FromContextOrDiscard(context.TODO())

	result, err := ParseFromEnv(logger)

	assert.NoError(t, err)
	assert.Equal(t, "8443", result.Port)
}

func Test_ParseFromEnv_InvalidPortRangeShouldUseDefaultValue(t *testing.T) {
	t.Setenv("WEBHOOK_PORT", "65536")
	logger := logr.FromContextOrDiscard(context.TODO())

	result, err := ParseFromEnv(logger)

	assert.NoError(t, err)
	assert.Equal(t, "8443", result.Port)
}

func Test_ParseFromEnv_ValidPort(t *testing.T) {
	t.Setenv("WEBHOOK_PORT", "1234")
	logger := logr.FromContextOrDiscard(context.TODO())

	result, err := ParseFromEnv(logger)

	assert.NoError(t, err)
	assert.Equal(t, "1234", result.Port)
}

func Test_ParseFromEnv_TLSEnabledUnsetDefaultFalse(t *testing.T) {
	logger := logr.FromContextOrDiscard(context.TODO())

	result, err := ParseFromEnv(logger)

	assert.NoError(t, err)
	assert.Equal(t, false, result.TLSEnabled)
}

func Test_ParseFromEnv_TLSEnabledInvalidValueDefaultFalse(t *testing.T) {
	t.Setenv("TLS_SERVER", "trruuee")
	logger := logr.FromContextOrDiscard(context.TODO())

	result, err := ParseFromEnv(logger)

	assert.NoError(t, err)
	assert.Equal(t, false, result.TLSEnabled)
}

func Test_ParseFromEnv_TLSEnabledValidTrueValue(t *testing.T) {
	t.Setenv("TLS_SERVER", "true")
	logger := logr.FromContextOrDiscard(context.TODO())

	result, err := ParseFromEnv(logger)

	assert.Equal(t, true, result.TLSEnabled)
	assert.Errorf(t, err, "failed parsing CA_CERT env variable")
}
