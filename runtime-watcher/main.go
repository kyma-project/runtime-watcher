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
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/go-logr/zapr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/kyma-project/runtime-watcher/skr/internal"
	"github.com/kyma-project/runtime-watcher/skr/internal/requestparser"
	"github.com/kyma-project/runtime-watcher/skr/internal/serverconfig"
	"github.com/kyma-project/runtime-watcher/skr/internal/watchermetrics"
)

//nolint:gochecknoglobals
var buildVersion = "not_provided"

func main() {
	var printVersion bool
	var development bool
	flag.BoolVar(&printVersion, "version", false, "Prints the watcher version and exits")
	flag.BoolVar(&development, "development", true, "Enable development mode")
	flag.Parse()

	if printVersion {
		msg := fmt.Sprintf("Runtime Watcher version: %s\n", buildVersion)
		_, err := os.Stdout.WriteString(msg)
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Set up zap logger
	zapConfig := zap.NewProductionConfig()
	if development {
		zapConfig = zap.NewDevelopmentConfig()
	}
	zapLogger, err := zapConfig.Build()
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer zapLogger.Sync()

	// Convert to logr.Logger using zapr
	logger := zapr.NewLogger(zapLogger.With(zap.String("component", "skr-webhook")))
	logger.Info("Starting Runtime Watcher", "Version", buildVersion)

	serverConfig, err := serverconfig.ParseFromEnv(logger)
	if err != nil {
		logger.Error(err, "necessary bootstrap settings missing")
		return
	}
	logger.Info("Server config successfully parsed: " + serverConfig.PrettyPrint())

	decoder := serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
	requestParser := requestparser.NewRequestParser(decoder)
	metrics := watchermetrics.NewMetrics()
	metrics.RegisterAll()
	logger.Info("All metrics registered")

	http.Handle("/metrics", promhttp.Handler())
	metricsServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", serverConfig.MetricsPort),
		ReadHeaderTimeout: internal.HTTPTimeout,
	}
	go func() {
		err := metricsServer.ListenAndServe()
		if err != nil {
			logger.Error(err, "failed to serve metrics endpoint")
		}
	}()
	logger.Info("Metrics server started")

	handler := internal.NewHandler(logger, serverConfig, *requestParser, *metrics)
	http.HandleFunc("/validate/", handler.Handle)
	server := http.Server{
		Addr:        fmt.Sprintf(":%d", serverConfig.Port),
		ReadTimeout: internal.HTTPTimeout,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
			MaxVersion: tls.VersionTLS13,
		},
	}
	logger.Info("Starting server for validation endpoint", "Port", serverConfig.Port)
	err = server.ListenAndServeTLS(serverConfig.TLSCertPath, serverConfig.TLSKeyPath)
	if err != nil {
		logger.Error(err, "error starting skr-webhook server")
		return
	}
}
