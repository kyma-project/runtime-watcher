package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"net/http"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

func main() {
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

	err = restClient.Get(context.Background(), client.ObjectKey{Name: "watcher-webhook-tls", Namespace: "default"}, &secret)
	if err != nil {
		return
	}

	//caCertPool := x509.NewCertPool()
	//caCertPool.AppendCertsFromPEM(secret.Data["ca.crt"])

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
		Timeout: time.Minute * 3,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      rootCertpool,
				Certificates: []tls.Certificate{certificate},
			},
		},
	}

	response, err := httpClient.Get("https://a.ab-test1.jellyfish.shoot.canary.k8s-hana.ondemand.com/misc")
	if err != nil {
		log.Fatalf("%v", err)
		return
	}

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
	response, err = httpClient.Post("https://a.ab-test1.jellyfish.shoot.canary.k8s-hana.ondemand.com/v1/example-listener/event", "application/json", bytes.NewBuffer(eventBytes))
	if err != nil {
		log.Fatalf("%v", err)
		return
	}
	fmt.Println(response.Status)
}
