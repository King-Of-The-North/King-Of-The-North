# Next.js merchant/admin web app. Build context = repo root (pnpm workspace shared
# with packages/design-system). NEXT_PUBLIC_API_URL is baked at build time because the
# browser (running on the host) calls the Gateway directly — default points at the
# host-exposed gateway on :8080.
FROM node:22-alpine

# Next/sharp on alpine wants libc compat.
RUN apk add --no-cache libc6-compat
RUN corepack enable

WORKDIR /app

# Workspace manifests first (better layer caching). All members are listed in
# pnpm-workspace.yaml, so their package.json files must exist for pnpm to resolve.
COPY pnpm-workspace.yaml pnpm-lock.yaml package.json ./
COPY frontend/package.json ./frontend/
COPY packages/design-system/package.json ./packages/design-system/
COPY mobile/package.json ./mobile/

# Install only the web app + its workspace deps (design-system); skip the mobile app.
RUN pnpm install --frozen-lockfile --filter "frontend..."

# Sources for the web app and the shared design system.
COPY packages/design-system ./packages/design-system
COPY frontend ./frontend

# Browser-facing gateway URL, baked into the client bundle at build time.
ARG NEXT_PUBLIC_API_URL=http://localhost:8080
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL

# Builds the design-system CSS (frontend prebuild) then the Next app.
RUN pnpm --filter frontend build

ENV NODE_ENV=production
EXPOSE 3000
CMD ["pnpm", "--filter", "frontend", "start"]
