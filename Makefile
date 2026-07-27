.PHONY: build test check install-local package-vscode package-jetbrains package-ide

build:
	go build -o autocurl ./cmd/autocurl

test:
	go test ./...

check:
	gofmt -w .
	go vet ./...
	go test ./...

install-local:
	go install ./cmd/autocurl

package-vscode:
	cd ide/vscode && npm ci && npm run package

package-jetbrains:
	cd ide/jetbrains && ./gradlew buildPlugin verifyPluginStructure verifyPluginProjectConfiguration

package-ide: package-vscode package-jetbrains
