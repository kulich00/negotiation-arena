FROM python:3.13-slim
RUN groupadd --system arena && useradd --system --gid arena arena
WORKDIR /app
COPY ml/requirements.txt ./requirements.txt
RUN pip install --no-cache-dir --index-url https://download.pytorch.org/whl/cpu -r requirements.txt
COPY ml/arena_model ./arena_model
COPY ml/server.py ./server.py
COPY ml/model ./model
USER arena
EXPOSE 8090
HEALTHCHECK --interval=15s --timeout=3s --retries=3 CMD ["python", "-c", "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8090/health', timeout=2).read()"]
ENTRYPOINT ["python", "server.py"]
