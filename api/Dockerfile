# Build stage (Go)
FROM golang:1.22-alpine AS gobuild
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN ls -la && ls cmd/ 2>/dev/null || echo "NO CMD DIR"
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

# Runtime stage (Node sidecar + Go binary)
FROM node:20-alpine
WORKDIR /app
COPY tools/package*.json ./tools/
RUN cd tools && npm ci --omit=dev
COPY tools/analyze-video.js ./tools/
COPY --from=gobuild /server /server
ENV ANALYZE_VIDEO_SCRIPT=/app/tools/analyze-video.js
EXPOSE 8080
CMD ["/server"]
