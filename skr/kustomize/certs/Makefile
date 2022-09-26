##@ Vars
PREFIX=localhost
ENV_DIR=$(KUSTOMIZE_DIR)/base/env
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
KUSTOMIZE_DIR = $(abspath $(shell pwd)/..)

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
gen-skr-cert: gen-ca-cert 
	$(CFSSL) gencert -ca $(CA_CERT_PEM) -ca-key $(CA_KEY_PEM) -config $(CFSSL_PROFILES_CONFIG) -profile=$(SKR_PROFILE) skr-cluster.json > cfssl-out.json
	jq -r .cert cfssl-out.json > $(SKR_CERT_PEM)
	jq -r .key cfssl-out.json > $(SKR_KEY_PEM)
	rm cfssl-out.json


.PHONY: write-secrets
write-secrets: gen-ca-cert gen-skr-cert
	export CA_CERT=$(shell cat $(CA_CERT_PEM) | sed 's/---//g' | base64) && \
	sed -ir "s/^[#]*\s*CA_CERT=.*/CA_CERT=$$CA_CERT/" $(ENV_DIR)/secret-keys.env
	export TLS_CERT=$(shell cat $(SKR_CERT_PEM) | sed 's/---//g' | base64) && \
	sed -ir "s/^[#]*\s*TLS_CERT=.*/TLS_CERT=$$TLS_CERT/" $(ENV_DIR)/secret-keys.env
	export TLS_KEY=$(shell cat $(SKR_KEY_PEM) | sed 's/---//g' | base64) && \
	sed -ir "s/^[#]*\s*TLS_KEY=.*/TLS_KEY=$$TLS_KEY/" $(ENV_DIR)/secret-keys.env


.PHONY: kustomize-deploy-test
kustomize-deploy-test:  
	kubectl apply -k $(KUSTOMIZE_DIR)/overlays/test


.PHONY: clean-up
clean-up:  
	rm *.csr *.pem

