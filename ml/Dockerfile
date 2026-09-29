FROM python:3.13-alpine
RUN addgroup -S arena && adduser -S arena -G arena
WORKDIR /app
COPY ml/arena_model ./arena_model
COPY ml/server.py ./server.py
COPY ml/model ./model
USER arena
EXPOSE 8090
HEALTHCHECK --interval=15s --timeout=3s --retries=3 CMD wget -qO- http://127.0.0.1:8090/health || exit 1
ENTRYPOINT ["python", "server.py"]
