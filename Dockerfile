# Stage 0: Build frontend
FROM node:25-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# Stage 1: Build Go binary
FROM golang:1.25-alpine AS backend
ENV GOCACHE=/root/.cache/go-build

# Add Maintainer Info
LABEL maintainer="William Floyd <github@notmy.space>"

# Set the Current Working Directory inside the container
WORKDIR /app
RUN apk add --no-cache bash git openssh libwebp gcc musl-dev
COPY go.mod go.sum ./
RUN --mount=type=cache,target="/root/.cache/go-build" go mod download
COPY main.go main.go
COPY backend/ backend/
RUN --mount=type=cache,target="/root/.cache/go-build" go build -ldflags="-extldflags -static" -o main . && mkdir -p /tmp

# Stage 2: Final image
FROM scratch
WORKDIR /
COPY --from=backend /app/main /app/main
COPY --from=frontend /app/dist /dist
# Go's multipart parser buffers uploads to /tmp; scratch has no /tmp
COPY --from=backend /tmp /tmp
ENTRYPOINT ["/app/main"]
