package main

import (
	"fmt"
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
