# Build frontend
FROM docker.io/library/node:18-alpine AS frontend
WORKDIR /build/frontend
COPY frontend/package*.json ./
COPY frontend/babel.config.js ./
COPY frontend/vue.config.js ./
COPY frontend/scripts ./scripts
RUN npm install
COPY frontend/public ./public
COPY frontend/src ./src
RUN npm run build

# Build backend
FROM docker.io/library/golang:1.24-alpine AS backend
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /build/static/dist ./static/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /web-ai ./cmd/server/

# Runtime
FROM scratch
COPY --from=backend /web-ai /web-ai
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
VOLUME ["/data"]
ENV PORT=8080
ENV DATA_DIR=/data
ENV CONFIG=/data/config.json
EXPOSE 8080
ENTRYPOINT ["/web-ai"]
