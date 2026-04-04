# Stage 1: Build the React application
FROM node:alpine AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm ci

# Copy source code
COPY . .

# Build the application
RUN npm run build

# Stage 2: Serve with nginx
FROM nginx:alpine

# Copy built assets from builder stage
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy custom nginx configuration with health endpoints
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Expose the same port the chart probes and services expect
EXPOSE 8080

# Start nginx
CMD ["nginx", "-g", "daemon off;"]
