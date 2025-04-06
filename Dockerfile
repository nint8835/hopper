FROM golang:1.24-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build .

FROM gcr.io/distroless/static AS bot

WORKDIR /bot
COPY --from=builder /build/hopper .

ENTRYPOINT [ "/bot/hopper" ]
CMD [ "run" ]
