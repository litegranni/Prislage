FROM node:22-bookworm AS build
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install
COPY . .
ARG VITE_API_BASE=http://localhost:3847
ARG VITE_API_TOKEN=dev-token-change-me
ENV VITE_API_BASE=$VITE_API_BASE VITE_API_TOKEN=$VITE_API_TOKEN
RUN npm run build

FROM node:22-bookworm-slim
WORKDIR /app
RUN npm install -g serve@14
COPY --from=build /app/dist ./dist
COPY public/capture.js ./dist/capture.js
EXPOSE 4188
CMD ["serve", "-s", "dist", "-l", "4188"]
