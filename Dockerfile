# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG SERVICE=agile-manager
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/${SERVICE}

FROM alpine:3.20

WORKDIR /app

COPY --from=build /out/server /app/server
COPY --from=build /src/web/static /app/web/static

EXPOSE 8080

ENTRYPOINT ["/app/server"]
