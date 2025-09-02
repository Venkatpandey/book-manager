generate:
	cd api && go generate ./...

unit_test:
	go test ./... -tags=unit --race --cover

integration_test:
	go test ./... -tags=integration

run:
	go run ./cmd/api