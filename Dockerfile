FROM golang:1.25 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /pheromone-server ./cmd/pheromone-server

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /pheromone-server /pheromone-server
EXPOSE 8081
ENTRYPOINT ["/pheromone-server", "serve"]
