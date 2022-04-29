version := 0.0.5
projectName := ugform

build: format compile push

format:
	go fmt ./...

compile:
	go build ugform.go

push:
	git add ugform.go
	git add README.md
	git add Makefile
	git add sample/main.go sample/go.mod
	git commit -m "$(M)"
	git tag v$(version)
	git push origin v$(version)
	git push origin master
