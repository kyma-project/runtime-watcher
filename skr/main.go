/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"net/http"
	"os"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/kyma-project/runtime-watcher/skr/internal"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type ServerParameters struct {
	port       int    // webhook server port
	certFile   string // path to TLS certificate for https
	keyFile    string // path to TLS key matching for certificate
	tlsEnabled bool   // indicates if TLS is enabled
}

const (
	defaultPort           = 8443
	defaultTLSEnabledMode = false
)

func serverParams(logger logr.Logger) ServerParameters {
	parameters := ServerParameters{}

	// port
	portEnv := os.Getenv("WEBHOOK_PORT")
	port, err := strconv.Atoi(portEnv)
	if err != nil {
		logger.V(1).Error(err, "failed parsing web-hook server port")
		parameters.port = defaultPort
	}
	parameters.port = port

	// tls
	tlsEnabledEnv := os.Getenv("TLS_ENABLED")
	tlsEnabled, err := strconv.ParseBool(tlsEnabledEnv)
	if err != nil {
		logger.V(1).Error(err, "failed parsing  tls flag")
		parameters.tlsEnabled = defaultTLSEnabledMode
	}
	parameters.tlsEnabled = tlsEnabled
	parameters.certFile = os.Getenv("TLS_CERT")
	parameters.keyFile = os.Getenv("TLS_KEY")
	return parameters
}

func main() {
	logger := ctrl.Log.WithName("skr-webhook")

	opts := zap.Options{
		Development: true,
	}

	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	params := serverParams(logger)

	restConfig := ctrl.GetConfigOrDie()

	restClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		logger.Error(err, "rest client could not be determined for skr-webhook")
		return
	}

	// handler
	handler := &internal.Handler{
		Client: restClient,
		Logger: logger,
	}
	http.HandleFunc("/validate/", handler.Handle)

	// server
	logger.Info("starting web server", "Port:", params.port)
	if params.tlsEnabled {
		err = http.ListenAndServeTLS(":"+strconv.Itoa(params.port), params.certFile,
			params.keyFile, nil)
	} else {
		err = http.ListenAndServe(":"+strconv.Itoa(params.port), nil)
	}
	if err != nil {
		logger.Error(err, "error starting skr-webhook server")
		return
	}
}
