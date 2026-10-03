# Stage 1: build. The Go toolchain is only needed here and does not get into the final image.
FROM golang:1.27.1-alpine AS build
WORKDIR /src

# Dependencies first: this layer is rebuilt only when go.mod or go.sum change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0 gives a static binary that runs without libc.
# -trimpath drops local paths, -s -w drop debug info: a smaller binary.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /shortener ./cmd/shortener

# Stage 2: run. Only the binary, no shell or package manager, non-root user.
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /shortener /shortener
EXPOSE 8080
ENTRYPOINT ["/shortener"]
