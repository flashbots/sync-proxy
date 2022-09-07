GIT_VER := $(shell git describe --tags --always --dirty="-dev")
ECR_URI := 223847889945.dkr.ecr.us-east-2.amazonaws.com/sync-proxy-name

all: clean build

v:
	@echo "Version: ${GIT_VER}"

clean:
	rm -rf sync-proxy build/

build:
	go build -ldflags "-X main.version=${GIT_VER}" -v -o sync-proxy .

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	gofmt -d ./
	go vet ./...
	staticcheck ./...

cover:
	go test -coverprofile=/tmp/go-sim-lb.cover.tmp ./...
	go tool cover -func /tmp/go-sim-lb.cover.tmp
	unlink /tmp/go-sim-lb.cover.tmp

cover-html:
	go test -coverprofile=/tmp/go-sim-lb.cover.tmp ./...
	go tool cover -html=/tmp/go-sim-lb.cover.tmp
	unlink /tmp/go-sim-lb.cover.tmp

build-for-docker:
	GOOS=linux go build -ldflags "-X main.version=${GIT_VER}" -v -o sync-proxy .

docker-image:
	DOCKER_BUILDKIT=1 docker build . -t sync-proxy
	docker tag sync-proxy:latest ${ECR_URI}:${GIT_VER}
	docker tag sync-proxy:latest ${ECR_URI}:latest

docker-push:
	docker push ${ECR_URI}:${GIT_VER}
	docker push ${ECR_URI}:latest

k8s-deploy:
	@echo "Checking if Docker image ${ECR_URI}:${GIT_VER} exists..."
	@docker manifest inspect ${ECR_URI}:${GIT_VER} > /dev/null || (echo "Docker image not found" && exit 1)
	kubectl set image deploy/deployment-sync-proxy app-sync-proxy=${ECR_URI}:${GIT_VER}
	kubectl rollout status deploy/deployment-sync-proxy
