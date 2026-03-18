# Stage 1: Build UI
FROM node:22-alpine AS ui-build
WORKDIR /app/ui
COPY ui/package*.json ./
RUN npm ci --legacy-peer-deps
COPY ui/ .
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.25.8-alpine AS go-build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 go build -o aap-server ./cmd/aap-server

# Stage 3: Final image
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=go-build /app/aap-server /usr/local/bin/
COPY --from=ui-build /app/ui/dist /app/static
COPY VERSION /app/VERSION
ENV AAP_STATIC_DIR=/app/static
ENV AAP_PORT=8080
EXPOSE 8080
USER 1001
ENTRYPOINT ["aap-server"]
