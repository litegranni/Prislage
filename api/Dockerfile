FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server && \
    CGO_ENABLED=0 go build -o /out/prislage ./cmd/prislage

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/server /out/prislage /app/
COPY migrations /app/migrations
ENV MIGRATIONS_DIR=/app/migrations
ENV HTTP_ADDR=:3847
EXPOSE 3847
CMD ["/app/server"]
