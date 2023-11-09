FROM golang:1.21
ADD . /app
WORKDIR /app
RUN go mod vendor
RUN CGO_ENABLED=0 GOOS=linux go build -o /asw-stundenplan cmd/*.go
WORKDIR /
ENTRYPOINT ["/app/asw-stundenplan"]