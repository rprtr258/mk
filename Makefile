mk: main.go
	@go build -o mk main.go

.PHONY: run
run: mk
	@./mk
