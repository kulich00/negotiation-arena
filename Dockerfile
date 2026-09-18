FROM node:22-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build

FROM golang:1.27-alpine AS backend
WORKDIR /src/backend
COPY backend/go.mod ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend /src/frontend/dist ./internal/webapp/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/arena ./cmd/server

FROM alpine:3.22
RUN addgroup -S arena && adduser -S arena -G arena
WORKDIR /app
COPY --from=backend /out/arena /app/arena
COPY backend/migrations /app/migrations
USER arena
EXPOSE 8080
HEALTHCHECK --interval=20s --timeout=3s --retries=3 CMD wget -qO- http://127.0.0.1:8080/health/live || exit 1
ENTRYPOINT ["/app/arena"]

