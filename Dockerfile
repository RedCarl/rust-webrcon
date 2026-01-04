FROM node:20-alpine AS web
WORKDIR /app
COPY package*.json ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY . .
RUN npm run build

FROM golang:1.23-alpine AS backend
WORKDIR /app
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download
COPY backend ./backend
RUN cd backend && CGO_ENABLED=0 GOOS=linux go build -o /out/server .

FROM alpine:3.19
WORKDIR /app
COPY --from=backend /out/server ./server
COPY --from=web /app/dist ./dist
ENV BIND_ADDR=0.0.0.0:3000
EXPOSE 3000
CMD ["./server"]
