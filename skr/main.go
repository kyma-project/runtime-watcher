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
	"github.com/kyma-project/runtime-watcher/skr/internal/serverconfig"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kyma-project/runtime-watcher/skr/internal"
)

func main() {
	logger := ctrl.Log.WithName("skr-webhook")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
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

	deserializer := serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()

	handler := &internal.Handler{
		Client:       restClient,
		Logger:       logger,
		Config:       config,
		Deserializer: deserializer,
	}
	http.HandleFunc("/validate/", handler.Handle)

	server := http.Server{
		Addr:        fmt.Sprintf(":%s", config.Port),
		ReadTimeout: internal.HTTPClientTimeout,
	}
	logger.Info("starting web server", "Port:", config.Port)
	if config.TLSEnabled {
		err = server.ListenAndServeTLS(config.TLSCert, config.TLSKey)
	} else {
		err = server.ListenAndServe()
	}
	if err != nil {
		logger.Error(err, "error starting skr-webhook server")
		return
	}
}
