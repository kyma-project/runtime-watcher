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
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/kyma-project/runtime-watcher/skr/internal"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	defaultPort           = 8443
	defaultTLSEnabledMode = false
)

func flagError(flagName string) error {
	return fmt.Errorf("failed parsing %s flag", flagName)
}

func serverParams(logger logr.Logger) (internal.ServerParameters, error) {
	parameters := internal.ServerParameters{}

	// port
	portEnv := os.Getenv("WEBHOOK_PORT")
	port, err := strconv.Atoi(portEnv)
	if err != nil {
		logger.V(1).Error(err, flagError("WEBHOOK_PORT").Error())
		parameters.Port = defaultPort
	}
	parameters.Port = port

	// tls server
	tlsServerEnv := os.Getenv("TLS_SERVER")
	parameters.TLSServer, err = strconv.ParseBool(tlsServerEnv)
	if err != nil {
		logger.V(1).Error(err, "failed parsing tls server flag")
		parameters.TLSServer = defaultTLSEnabledMode
	}

	// tls callback
	tlsCallbackEnv := os.Getenv("TLS_CALLBACK")
	parameters.TLSCallback, err = strconv.ParseBool(tlsCallbackEnv)
	if err != nil {
		logger.V(1).Error(err, "failed parsing tls callback flag")
		parameters.TLSCallback = defaultTLSEnabledMode
	}

	if parameters.TLSServer {
		// CA cert
		parameters.CACert = os.Getenv("CA_CERT")
		if parameters.CACert == "" {
			return parameters, flagError("CA_CERT")
		}
		// client cert
		parameters.TLSCert = os.Getenv("TLS_CERT")
		if parameters.TLSCert == "" {
			return parameters, flagError("TLS_CERT")
		}
		// client key
		parameters.TLSKey = os.Getenv("TLS_KEY")
		if parameters.TLSKey == "" {
			return parameters, flagError("TLS_KEY")
		}
	}
	return parameters, nil
}

func main() {
	logger := ctrl.Log.WithName("skr-webhook")

	opts := zap.Options{
		Development: true,
	}

	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	params, err := serverParams(logger)
	if err != nil {
		logger.Error(err, "necessary bootstrap settings missing")
		return
	}

	restConfig := ctrl.GetConfigOrDie()

	restClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		logger.Error(err, "rest client could not be determined for skr-webhook")
		return
	}

	// handler
	handler := &internal.Handler{
		Client:     restClient,
		Logger:     logger,
		Parameters: params,
	}
	http.HandleFunc("/validate/", handler.Handle)

	// server
	server := http.Server{
		Addr:        fmt.Sprintf(":%s", strconv.Itoa(params.Port)),
		ReadTimeout: internal.HTTPClientTimeout,
	}
	logger.Info("starting web server", "Port:", params.Port)
	if params.TLSServer {
		err = server.ListenAndServeTLS(params.TLSCert, params.TLSKey)
	} else {
		err = server.ListenAndServe()
	}
	if err != nil {
		logger.Error(err, "error starting skr-webhook server")
		return
	}
}
