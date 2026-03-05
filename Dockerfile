FROM golang:1.22 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/nard ./cmd/nard

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/nard /nard
ENTRYPOINT ["/nard"]
