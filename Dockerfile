FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y git build-essential gcc libc6-dev pkg-config libsqlite3-dev

RUN mkdir -p /home/appuser && \
    echo "appuser:x:1000:1000::/home/appuser:/usr/sbin/nologin" >> /etc/passwd && \
    echo "appuser:x:1000:" >> /etc/group

WORKDIR /app

COPY main .

RUN chmod +x main && mkdir -p data

EXPOSE 8080

CMD ["./main"]