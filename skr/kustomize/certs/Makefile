##@ Vars
PREFIX=localhost
CA=$(PREFIX)-ca
CA_CERT_PEM=$(CA).pem
CA_KEY_PEM=$(CA)-key.pem
CFSSL_PROFILES_CONFIG=cfssl-config.json
SKR_PROFILE=client
SKR=$(PREFIX)-skr
SKR_CERT_PEM=$(SKR).pem
SKR_KEY_PEM=$(SKR)-key.pem

.PHONY: all
all: cfssl gen-ca-cert gen-skr-cert

CFSSL ?= $(LOCALBIN)/cfssl
CFSSLJSON ?= $(LOCALBIN)/cfssljson

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Location to write certs to
KUSTOMIZE_DIR ?= $(abspath $(shell pwd)/../kustomize)
$(KUSTOMIZE_DIR):
	mkdir -p $(KUSTOMIZE_DIR)

cfssl: $(CFSSL) ## Download cfssl locally if necessary.
$(CFSSL): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/cloudflare/cfssl/cmd/cfssl@latest


.PHONY: gen-ca-cert
gen-ca-cert: cfssl
	$(CFSSL) gencert -initca ca.json > cfssl-out.json
	jq -r .cert cfssl-out.json > $(CA_CERT_PEM)
	jq -r .key cfssl-out.json > $(CA_KEY_PEM)
	rm cfssl-out.json

.PHONY: gen-skr-cert
gen-skr-cert: cfssl gen-ca-cert 
	$(CFSSL) gencert -ca $(CA_CERT_PEM) -ca-key $(CA_KEY_PEM) -config $(CFSSL_PROFILES_CONFIG) -profile=$(SKR_PROFILE) skr-cluster.json > cfssl-out.json
	jq -r .cert cfssl-out.json > $(SKR_CERT_PEM)
	jq -r .key cfssl-out.json > $(SKR_KEY_PEM)
	rm cfssl-out.json

.PHONY: kustomize
kustomize:  
	kubectl kustomize $(KUSTOMIZE_DIR)


.PHONY: clean-up
clean-up:  
	rm *.csr *.pem

