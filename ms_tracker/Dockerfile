FROM golang:1.25

WORKDIR /app

COPY go.mod ./
COPY go.sum* ./
RUN go mod download

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/tracker

EXPOSE 8080
CMD ["./main"]
