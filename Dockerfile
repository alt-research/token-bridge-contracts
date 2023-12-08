FROM node:18.15.0-slim AS builder
WORKDIR /app
COPY . .
RUN apt update && apt install git -y
RUN apt install software-properties-common -y && add-apt-repository ppa:deadsnakes/ppa && apt install python3 -y
RUN apt install chromium -y
RUN apt install build-essential -y
RUN yarn install
RUN yarn build

ENTRYPOINT [ "yarn run test:tokenbridge:deployment" ]
