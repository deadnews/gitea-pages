.PHONY: alpha build bumped check goreleaser pc release run test up update

default: check

check: pc test
pc:
	prek run -a
test:
	go test -race -covermode=atomic -coverprofile=coverage.txt ./...
	@go tool cover -func=coverage.txt | tail -1

update: up up-ci
up:
	go get -u -t ./...
	go mod tidy
	go mod verify
up-ci:
	prek update
	pindock run --update
	pinact run --update

run:
	go run ./cmd/gitea-pages

build:
	go build -ldflags=-s -o ./dist/ ./...

goreleaser:
	goreleaser --clean --snapshot --skip=publish

bumped:
	git cliff --bumped-version

# make alpha TAG=$(git cliff --bumped-version)-alpha.0
alpha: check
	git tag -a $(TAG) -m "chore(release): $(TAG)"
	git push origin $(TAG)

# make release TAG=$(git cliff --bumped-version)
release: check
	git cliff -o CHANGELOG.md --tag $(TAG)
	prek run --files CHANGELOG.md || prek run --files CHANGELOG.md
	git add CHANGELOG.md
	git commit -m "chore(release): prepare for $(TAG)"
	git push
	git tag -a $(TAG) -m "chore(release): $(TAG)"
	git push origin $(TAG)
