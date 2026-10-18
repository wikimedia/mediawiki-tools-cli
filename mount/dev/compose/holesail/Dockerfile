FROM node:24-bookworm-slim
WORKDIR /opt/holesail
COPY package.json package-lock.json ./
RUN npm ci --omit=dev --no-audit --no-fund && npm cache clean --force
USER node
ENTRYPOINT ["/opt/holesail/node_modules/.bin/holesail"]
