FROM node:18.15.0-slim AS builder
WORKDIR /app
COPY . .
RUN yarn install
RUN yarn build

ENTRYPOINT [ "yarn run test:tokenbridge:deployment" ]
