package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kyma-project/kyma-watcher/kcp/pkg/types"
	"github.com/kyma-project/kyma-watcher/skr/pkg/config"
	"io/ioutil"
	"net/http"
	"os"
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
	// check env for relevant values
	portEnv := os.Getenv("WEBHOOK_PORT")
	port := 8443
	var err error

	if portEnv != "" {
		port, err = strconv.Atoi(portEnv)
		if err != nil {
			klog.Error("Error parsing Webhook server port: ", err.Error())
		}
	}

	parameters.port = port

	t := os.Getenv("TLS_ENABLED")
	tlsEnabled := false

	if t != "" {
		tlsEnabled, err = strconv.ParseBool(t)
		if err != nil {
			klog.Error("Error parsing tls: ", err.Error())
		}
	}
	parameters.tlsEnabled = tlsEnabled

	parameters.certFile = os.Getenv("TLS_CERT")
	parameters.keyFile = os.Getenv("TLS_KEY")

	if err != nil {
		klog.Fatalf("Config build: ", err.Error())
	}

	http.HandleFunc("/validate", func(handler http.ResponseWriter, req *http.Request) {
		fmt.Println("send web requests to kcp")

		watcherEvent := &types.WatcherEvent{
			KymaCr:    "kyma-sample",
			Namespace: "default",
			Name:      "manifestkyma-sample",
		}
		postBody, _ := json.Marshal(watcherEvent)

		responseBody := bytes.NewBuffer(postBody)

		kcpIp := os.Getenv("KCP_IP")
		kcpPort := os.Getenv("KCP_PORT")
		contract := os.Getenv("KCP_CONTRACT")
		component := os.Getenv("COMPONENT")

		url := fmt.Sprintf("http://%s:%s/%s/%s/%s", kcpIp, kcpPort, contract, component, config.EventEndpoint)
		fmt.Println("url" + url)
		resp, err := http.Post(url, "application/json", responseBody)

		if err != nil {
			fmt.Println("error" + err.Error())
		}
		fmt.Println(resp)
		body, err := ioutil.ReadAll(req.Body)
		err = ioutil.WriteFile("/tmp/request", body, 0644)
		if err != nil {
			panic(err.Error())
		}
	})

	if parameters.tlsEnabled {
		klog.Fatal(http.ListenAndServeTLS(":"+strconv.Itoa(parameters.port), parameters.certFile, parameters.keyFile, nil))
	} else {
		klog.Fatal(http.ListenAndServe(":"+strconv.Itoa(parameters.port), nil))
	}
}
