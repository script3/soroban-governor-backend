test:
	go test ./...

run-indexer:
	go run cmd/indexer/main.go

run-api:
	go run cmd/api/main.go

build-docker:
	docker build -t governor-indexer -f ./docker/Dockerfile.indexer --platform linux/amd64 .
	docker build -t governor-api -f ./docker/Dockerfile.api --platform linux/amd64 .
