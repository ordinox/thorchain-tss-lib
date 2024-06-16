
all: protob test

########################################
### Protocol Buffers

protob:
	@echo "--> Building Protocol Buffers"
	@for file in shared message ecdsa-keygen ecdsa-signing ecdsa-signature ecdsa-resharing eddsa-keygen eddsa-signing eddsa-signature eddsa-resharing; do \
		echo "Generating $$file.pb.go" ; \
		protoc --go_out=module=gitlab.com/thorchain/tss/tss-lib:. ./protob/$$file.proto ; \
	done

build: protob
	go fmt ./...

########################################
### Testing

test:
	@echo "--> Running Unit Tests"
	go test -timeout 60m -v -coverprofile=coverage.out ./...

test_race:
	@echo "--> Running Unit Tests (with Race Detection)"
	go test -timeout 60m -race -v -coverprofile=coverage.out ./...

########################################

# To avoid unintended conflicts with file names, always add to .PHONY
# # unless there is a reason not to.
# # https://www.gnu.org/software/make/manual/html_node/Phony-Targets.html
.PHONY: protob build test test_race test
