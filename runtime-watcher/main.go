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

	"github.com/kyma-project/runtime-watcher/skr/internal/parser"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kyma-project/runtime-watcher/skr/internal"
	"github.com/kyma-project/runtime-watcher/skr/internal/serverconfig"
)

var buildVersion string = "unknown"

func main() {
	var printVersion bool
	flag.BoolVar(&printVersion, "version", false, "Prints the watcher version and exits")

	logger := ctrl.Log.WithName("skr-webhook")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if printVersion {
		fmt.Printf("Runtime Watcher version: %s\n", buildVersion)
		os.Exit(1)
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	config, err := serverconfig.ParseFromEnv(logger)
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

	requestParser := parser.NewRequestParser(serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer())
	handler := internal.NewHandler(restClient, logger, config, *requestParser)
	http.HandleFunc("/validate/", handler.Handle)

	server := http.Server{
		Addr:        fmt.Sprintf(":%d", config.Port),
		ReadTimeout: internal.HTTPSClientTimeout,
	}
	logger.Info("starting web server", "Port:", config.Port)
	err = server.ListenAndServeTLS(config.TLSCertPath, config.TLSKeyPath)

	if err != nil {
		logger.Error(err, "error starting skr-webhook server")
		return
	}
}
