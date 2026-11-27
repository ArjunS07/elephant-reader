FROM node:22-alpine AS web
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY index.html vite.config.ts tsconfig.json tsconfig.app.json tsconfig.node.json ./
COPY public ./public
COPY src ./src
ARG VITE_ELEPHANT_API_TOKEN
ENV VITE_ELEPHANT_API_TOKEN=$VITE_ELEPHANT_API_TOKEN
RUN npm run build

FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/elephant ./cmd/elephant

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
  && mkdir -p /data
WORKDIR /app
COPY --from=build /out/elephant /app/elephant
COPY --from=web /src/dist /app/dist
ENV PORT=8080 \
    STATIC_DIR=/app/dist \
    DATABASE_PATH=/data/elephant.db \
    ELEPHANT_TIMEZONE=America/Los_Angeles
EXPOSE 8080
CMD ["/app/elephant", "serve"]
