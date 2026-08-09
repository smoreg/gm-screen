# --- build ---
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOFLAGS=-trimpath go build -ldflags="-s -w" -o /out/gm-screen .

# --- run ---
FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=build /out/gm-screen /app/gm-screen
COPY web /app/web
ENV GM_WEB_DIR=/app/web PORT=8777
EXPOSE 8777
# ANTHROPIC_API_KEY передаётся на рантайме (--env-file), в образ НЕ пекётся.
USER nonroot:nonroot
ENTRYPOINT ["/app/gm-screen"]
