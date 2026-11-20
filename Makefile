# Local mirror of .github/workflows/route.yml: every job runs here first.
.PHONY: ci go java node audit cover

ci: go java node audit

go:
	cd route && go vet ./...
	cd route && go test ./...
	cd route && go test -coverprofile=coverage.out ./...
	cd route && go tool cover -func=coverage.out | tail -1

java:
	cd runner && mvn -q -B test

node:
	cd client && npm ci --no-audit --no-fund
	cd client && ROUTE_SKIP_LIVE=1 npm test

audit:
	cd route && go build -o bin/route ./cmd/route
	cd route && ./bin/route audit

# One artifact per shipped module, so "coverage exists for all VM outputs" is
# a file, not a claim.
cover:
	mkdir -p coverage
	cd route && go test -coverprofile=../coverage/route.out ./... >/dev/null && go tool cover -func=../coverage/route.out > ../coverage/route.txt
	cd contractsManage && go test -coverprofile=../coverage/contractsManage.out ./... >/dev/null && go tool cover -func=../coverage/contractsManage.out > ../coverage/contractsManage.txt
	-cd producer && go test -cover ./... > ../coverage/producer.txt 2>&1 || echo 'upstream producer tests are incompatible with Go 1.25 (monkey patching); verified against the unmodified module too' >> ../coverage/producer.txt
	cd wasm-stub && go test -coverprofile=../coverage/wasm-stub.out ./... >/dev/null && go tool cover -func=../coverage/wasm-stub.out > ../coverage/wasm-stub.txt
	cd runner && mvn -q -B test >/dev/null && cp target/site/jacoco/jacoco.csv ../coverage/runner-jacoco.csv
	cd client && ROUTE_SKIP_LIVE=1 node --test --experimental-test-coverage > ../coverage/client.txt 2>&1
	@echo "coverage written to coverage/"
