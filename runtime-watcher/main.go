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

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kyma-project/runtime-watcher/skr/internal"
	"github.com/kyma-project/runtime-watcher/skr/internal/requestparser"
	"github.com/kyma-project/runtime-watcher/skr/internal/serverconfig"
	"github.com/kyma-project/runtime-watcher/skr/internal/watchermetrics"
)

//nolint:gochecknoglobals
var buildVersion = "not_provided"

func main() {
	var printVersion bool
	flag.BoolVar(&printVersion, "version", false, "Prints the watcher version and exits")
	logger := ctrl.Log.WithName("skr-webhook").V(0)
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	if printVersion {
		msg := fmt.Sprintf("Runtime Watcher version: %s\n", buildVersion)
		_, err := os.Stdout.WriteString(msg)
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	logger.Info("Starting Runtime Watcher", "Version:", buildVersion)

	config, err := serverconfig.ParseFromEnv(logger)
	if err != nil {
		logger.Error(err, "necessary bootstrap settings missing")
		return
	}
	logger.Info("Server config successfully parsed: " + config.PrettyPrint())

	restConfig := ctrl.GetConfigOrDie()
	restClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		logger.Error(err, "rest client could not be determined for skr-webhook")
		return
	}
	logger.Info("REST client initialized")

	decoder := serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
	requestParser := requestparser.NewRequestParser(decoder)
	metrics := watchermetrics.NewMetrics()
	metrics.RegisterAll()
	logger.Info("All metrics registered")

	http.Handle("/metrics", promhttp.Handler())
	metricsServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", config.MetricsPort),
		ReadHeaderTimeout: internal.HTTPTimeout,
	}
	go func() {
		err = metricsServer.ListenAndServe()
		if err != nil {
			logger.Error(err, "failed to serve metrics endpoint")
		}
	}()
	logger.Info("Metrics server started")

	handler := internal.NewHandler(restClient, logger, config, *requestParser, *metrics)
	http.HandleFunc("/validate/", handler.Handle)
	server := http.Server{
		Addr:        fmt.Sprintf(":%d", config.Port),
		ReadTimeout: internal.HTTPTimeout,
	}
	logger.Info("Starting server for validation endpoint", "Port:", config.Port)
	err = server.ListenAndServeTLS(config.TLSCertPath, config.TLSKeyPath)
	if err != nil {
		logger.Error(err, "error starting skr-webhook server")
		return
	}
}
