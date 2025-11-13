FROM node:20-alpine

WORKDIR /app

COPY package*.json ./
RUN npm install --omit=dev

COPY public ./public
COPY server.mjs .

ENV PORT=3000
EXPOSE 3000

CMD ["npm", "start"]

