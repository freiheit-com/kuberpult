FROM alpine:3.21

ARG UID=1000

RUN apk --update add --no-cache nodejs npm
RUN npm install -g pnpm@8.15.9

EXPOSE 3000

RUN adduser --disabled-password --gecos "" --home "/kp" --uid ${UID} kp

RUN chown -R kp:kp /kp

USER kp

CMD [ "sh","-c","cd /kp && pnpm i && pnpm start" ]
