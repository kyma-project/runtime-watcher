package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"log"
	"net/http"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
)

const timeout = time.Second * 3

//nolint:funlen
func main() {
	os.Setenv("KUBECONFIG", "/Users/D063994/SAPDevelop/go/kubeconfigs/kcp.yaml")
	os.Setenv("KUBECONFIG", "/Users/d063994/.kube/config")
	restConfig, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		log.Fatalf("could not get rest config: %v", err)
	}

	restClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return
	}

	secret := v1.Secret{}

	err = restClient.Get(context.Background(), client.ObjectKey{
		Name:      "watcher-webhook-tls",
		Namespace: "default",
	}, &secret)
	if err != nil {
		return
	}

	certificate, err := tls.X509KeyPair(secret.Data["tls.crt"], secret.Data["tls.key"])
	if err != nil {
		log.Fatalf("could not load certificate: %v", err)
	}

	publicPemBlock, _ := pem.Decode(secret.Data["ca.crt"])
	rootPubCrt, errParse := x509.ParseCertificate(publicPemBlock.Bytes)
	if errParse != nil {
		log.Fatalf("failed to parse public key: %v", errParse)
	}
	rootCertpool := x509.NewCertPool()
	rootCertpool.AddCert(rootPubCrt)

	httpClient := http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			//nolint:gosec
			TLSClientConfig: &tls.Config{
				RootCAs:      rootCertpool,
				Certificates: []tls.Certificate{certificate},
			},
		},
	}

	// http bin
	response, err := httpClient.Get("https://wat.j2fmn4e1n7.jellyfish.shoot.canary.k8s-hana.ondemand.com/v1/httpbin")
	if err != nil {
		log.Fatalf("%v", err)
		return
	}
	log.Println(response.Status)
	_ = response.Body.Close()

	event := types.WatchEvent{
		Owner:      client.ObjectKey{Name: "example-owner", Namespace: "example-owner-ns"},
		Watched:    client.ObjectKey{Name: "example-watched", Namespace: "example-watched-ns"},
		WatchedGvk: metav1.GroupVersionKind{Kind: "example-kind", Group: "example-group", Version: "example-version"},
	}
	eventBytes, err := json.Marshal(event)
	if err != nil {
		log.Fatalf("%v", err)
		return
	}

	// example listener
	resp, err := httpClient.Post(
		"https://wat.j2fmn4e1n7.jellyfish.shoot.canary.k8s-hana.ondemand.com/v1/example-listener/event",
		"application/json",
		bytes.NewBuffer(eventBytes))
	if err != nil {
		log.Fatalf("%v", err)
		return
	}
	defer resp.Body.Close()
	log.Println(response.Status)
}
