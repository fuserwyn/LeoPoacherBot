.PHONY: build run clean

build:
	docker build -t leo-bot .

run: build
	docker run --env-file .env --rm leo-bot

clean:
	docker rmi leo-bot
