lint:
	golangci-lint run -c ./golangci.yml ./...

test:
	go test ./... -v --cover -skip TestSQLRun

jstypes:
	go run ./plugins/jsvm/internal/types/types.go

test-report:
	go test ./... -v --cover -coverprofile=coverage.out -skip TestSQLRun
	go tool cover -html=coverage.out
