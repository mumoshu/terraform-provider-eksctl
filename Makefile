.DEFAULT_GOAL := build
OS            := $(shell go env GOOS)
ARCH          := $(shell go env GOARCH)
# PLUGIN_PATH   ?= ${HOME}/.terraform.d/plugins/${OS}_${ARCH}
PLUGIN_PATH   ?= ${HOME}/.terraform.d/plugins/example.com/myorg/eksctl/0.8.6/${OS}_${ARCH}/
PLUGIN_NAME   := terraform-provider-eksctl
DIST_PATH     := dist/${OS}_${ARCH}
GO_PACKAGES   := $(shell go list ./... | grep -v /vendor/)
GO_FILES      := $(shell find . -type f -name '*.go')


.PHONY: all
all: test build

.PHONY: test
test: test-all

.PHONY: test-all
test-all:
	@TF_ACC=1 go test -v -race $(GO_PACKAGES)

${DIST_PATH}/${PLUGIN_NAME}: ${GO_FILES}
	mkdir -p $(DIST_PATH); \
	go build -o $(DIST_PATH)/${PLUGIN_NAME}

.PHONY: build
build: ${DIST_PATH}/${PLUGIN_NAME}

.PHONY: install
install: build
	mkdir -p $(PLUGIN_PATH); \
	rm -rf $(PLUGIN_PATH)/${PLUGIN_NAME}; \
	install -m 0755 $(DIST_PATH)/${PLUGIN_NAME} $(PLUGIN_PATH)/${PLUGIN_NAME}

# Set TF_LOG=DEBUG to enable debug logs from the provider
# Setting TF_LOG_PATH would also help
.PHONY: example/plan
example/plan:
	mkdir -p examples/terraform.d/plugins
	env PLUGIN_PATH=examples/terraform.d/plugins/$(OS)_$(ARCH) make install
	cd examples; terraform init; terraform plan

.PHONY: example/apply
example/apply:
	mkdir -p examples/terraform.d/plugins
	env PLUGIN_PATH=examples/terraform.d/plugins/$(OS)_$(ARCH) make install
	cd examples; terraform init; terraform apply

.PHONY: example/destroy
example/destroy:
	mkdir -p examples/terraform.d/plugins
	env PLUGIN_PATH=examples/terraform.d/plugins/$(OS)_$(ARCH) make install
	cd examples; terraform init; terraform destroy

.PHONY: clean
clean:
	rm -rf ${DIST_PATH}/*
