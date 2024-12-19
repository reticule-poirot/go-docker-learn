FROM golang:1.23.4

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY hello.go .

RUN go build -o myserver

EXPOSE 8000

CMD ["/build/myserver"]
