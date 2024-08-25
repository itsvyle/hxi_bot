FROM golang:1.22.6-alpine AS builder
WORKDIR /app

COPY / /app
RUN go mod download && go mod verify

RUN --mount=type=cache,target=/root/.docker-cache/go-build \ 
    go build -o /out/discord_bot_exe .

FROM alpine:3.19.1

WORKDIR /app

COPY --from=builder /out/discord_bot_exe /app/discord_bot_exe

ARG DOCKER_USER=default_user
RUN addgroup -S $DOCKER_USER && adduser -S $DOCKER_USER -G $DOCKER_USER
USER $DOCKER_USER


CMD ["/app/discord_bot_exe"]