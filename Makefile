version := 0.0.2
projectName := ugform

build: format compile release

format:
	go fmt ./...

compile:
	go build ugform.go

release:
	git add ugform.go
	git add README.md
	git add Makefile
	git add sample/main.go sample/go.mod
	git commit -m "$(M)"
	git tag v$(version)
	git push origin v$(version)
	git push origin master
