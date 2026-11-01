FROM debian:12 AS test
COPY . /src

FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /discord-poller ./cmd/discord-poller

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /discord-poller /discord-poller
ENTRYPOINT ["/discord-poller"]
