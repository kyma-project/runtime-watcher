package main

import (
	"github.com/kyma-project/kyma-watcher/skr/webhook/internal"
	"github.com/kyma-project/manifest-operator/operator/pkg/util"
	"net/http"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"

	"k8s.io/klog/v2"
)

type ServerParameters struct {
	port       int    // webhook server port
	certFile   string // path to TLS certificate for https
	keyFile    string // path to TLS key matching for certificate
	tlsEnabled bool   // indicates if TLS is enabled
}

var parameters ServerParameters

func main() {
	logger := ctrl.Log.WithName("skr-webhook")
	var err error

	// port
	portEnv := os.Getenv("WEBHOOK_PORT")
	defaultPort := 8443
	if portEnv != "" {
		defaultPort, err = strconv.Atoi(portEnv)
		if err != nil {
			logger.Error(err, "Error parsing Webhook server port")
		}
	}
	parameters.port = defaultPort

	// tls
	tlsEnabledEnv := os.Getenv("TLS_ENABLED")
	tlsEnabled := false
	if tlsEnabledEnv != "" {
		tlsEnabled, err = strconv.ParseBool(tlsEnabledEnv)
		if err != nil {
			logger.Error(err, "Error parsing tls flag")
		}
	}
	parameters.tlsEnabled = tlsEnabled
	parameters.certFile = os.Getenv("TLS_CERT")
	parameters.keyFile = os.Getenv("TLS_KEY")

	// rest client
	restConfig, err := util.GetConfig("", "")
	if err != nil {
		logger.Error(err, "rest config could not be determined for skr-webhook")
		return
	}
	client, err := client.New(restConfig, client.Options{})
	if err != nil {
		logger.Error(err, "rest client could not be determined for skr-webhook")
		return
	}

	// handler
	handler := &internal.Handler{
		Client: client,
		Logger: logger,
	}
	http.HandleFunc("/validate", handler.Handle)

	// server
	if parameters.tlsEnabled {
		klog.Fatal(http.ListenAndServeTLS(":"+strconv.Itoa(parameters.port), parameters.certFile,
			parameters.keyFile, nil))
	} else {
		klog.Fatal(http.ListenAndServe(":"+strconv.Itoa(parameters.port), nil))
	}
}
