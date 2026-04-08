NAME = replikator
DOCKER_REPO = cloudresty/${NAME}
DOCKER_TAG ?= test
VERSION ?= dev
GIT_COMMIT ?= unknown
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

GO_CMD = cd app && go
DOCKER_BUILD = docker build

.PHONY: help build build-local run shell clean test lint vet fmt-check docker-build docker-push manifest

help: ## Show list of make targets and their description.
	@grep -E '^[%a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build-local: ## Build the binary locally.
	@echo "Building ${NAME} locally..."
	@${GO_CMD} build -ldflags="-s -w -X main.version=${VERSION} -X main.gitCommit=${GIT_COMMIT} -X main.buildDate=${BUILD_DATE}" -o bin/${NAME} ./cmd/replikator

build: docker-build ## Build a docker image locally.

docker-build: ## Build Docker image.
	@echo "Building Docker image..."
	@${DOCKER_BUILD} \
		--platform linux/amd64,linux/arm64 \
		--pull \
		--force-rm \
		--tag ${DOCKER_REPO}:${DOCKER_TAG} \
		--file build/Dockerfile \
		--build-arg VERSION=${VERSION} \
		--build-arg GIT_COMMIT=${GIT_COMMIT} \
		--build-arg BUILD_DATE=${BUILD_DATE} \
		.

docker-build-no-platform: ## Build Docker image without multi-platform (faster local build).
	@echo "Building Docker image (no platform)..."
	@${DOCKER_BUILD} \
		--force-rm \
		--tag ${DOCKER_REPO}:${DOCKER_TAG} \
		--file build/Dockerfile \
		--build-arg VERSION=${VERSION} \
		--build-arg GIT_COMMIT=${GIT_COMMIT} \
		--build-arg BUILD_DATE=${BUILD_DATE} \
		.

test: ## Run all tests.
	@${GO_CMD} test -v -race ./...

test-coverage: ## Run all tests with coverage.
	@${GO_CMD} test -v -race -coverprofile=coverage.out ./...
	@${GO_CMD} tool cover -html=coverage.out -o coverage.html

lint: ## Run golangci-lint.
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@${GO_CMD} vet ./...
	@golangci-lint run --timeout=5m

fmt-check: ## Check code formatting.
	@${GO_CMD} fmt ./...
	@git diff --exit-code

vet: ## Run go vet.
	@${GO_CMD} vet ./...

run: ## Run docker image locally.
	@docker run \
		--rm \
		--name ${NAME} \
		--hostname ${NAME}-${DOCKER_TAG} \
		--platform linux/amd64 \
		${DOCKER_REPO}:${DOCKER_TAG}

shell: ## Run docker image in a shell locally.
	@docker run \
		--rm \
		--interactive \
		--tty \
		--name ${NAME} \
		--hostname ${NAME}-${DOCKER_TAG} \
		--platform linux/amd64 \
		--entrypoint /bin/sh \
		${DOCKER_REPO}:${DOCKER_TAG}

docker-push: ## Push docker image to registry.
	@docker push ${DOCKER_REPO}:${DOCKER_TAG}

docker-push-latest: ## Push docker image with latest tag.
	@docker tag ${DOCKER_REPO}:${DOCKER_TAG} ${DOCKER_REPO}:latest
	@docker push ${DOCKER_REPO}:latest

clean: ## Remove local build artifacts and docker images.
	@rm -rf bin/
	@if docker images -q ${DOCKER_REPO} | grep -q .; then \
		docker rmi $$(docker images -q ${DOCKER_REPO}:*) 2>/dev/null || true; \
	else echo "INFO: No images found for '${DOCKER_REPO}'"; fi

clean-build: ## Remove docker build cache.
	@docker builder prune -af || true

manifest: ## Generate YAML manifests (placeholder for kustomize).
	@echo "Manifests are in deploy/yaml/"
	@ls -la deploy/yaml/

deploy-kind: ## Deploy to kind cluster for testing.
	@kubectl apply -f deploy/yaml/single-mode.yaml
	@kubectl wait --for=condition=available deployment/replikator -n replikator --timeout=120s

undeploy-kind: ## Remove from kind cluster.
	@kubectl delete -f deploy/yaml/single-mode.yaml --wait

logs-kind: ## Show replikator logs from kind.
	@kubectl logs -n replikator -l app.kubernetes.io/name=replikator -f

.PHONY: all
all: fmt-check vet test lint build