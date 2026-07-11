FROM golang:1.26.3-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# api/ is gitignored — generate Swagger docs before build
RUN go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go --output api

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /bin/server /server

EXPOSE 8082

ENTRYPOINT ["/server"]
