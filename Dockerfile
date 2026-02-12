# -- Build stage --
FROM oven/bun:1 AS build

WORKDIR /app

COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

COPY tsup.config.ts tsconfig.json ./
COPY src/ src/
RUN bun run build

# -- Production stage --
FROM node:24-alpine

RUN apk add --no-cache git git-lfs openssh-client

WORKDIR /app

COPY package.json bun.lock ./
COPY --from=build /app/node_modules/ node_modules/
COPY --from=build /app/dist/ dist/
COPY bin/ bin/

ENTRYPOINT ["node", "bin/gogo"]
