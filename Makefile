.PHONY: ci-container lint

lint:
	golangci-lint run -v --timeout 180s

unittest:
	go test ./...

ci-container:
	DOCKER_BUILDKIT=1 docker build --pull -t gcr.io/windmill-test-containers/ephemerator-ci -f .circleci/Dockerfile .
	docker push gcr.io/windmill-test-containers/ephemerator-ci
