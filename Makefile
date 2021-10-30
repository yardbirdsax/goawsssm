.PHONY: mock
mock: 
	~/go/bin/mockgen -source=client/ssm.go -destination=mock/ssm.go -package mock
	~/go/bin/mockgen -source=client/exec.go -destination=mock/exec.go -package mock
	make tidy

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: test
test:
	go test ./session/...

.PHONY: integration-test
integration-test:
	go test -v -count=1 -timeout=30m ./integration_test/... 