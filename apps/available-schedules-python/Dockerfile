FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY main.py .
ENV SERVICE_NAME=available-schedules ERROR_RATE=0.01 EXTRA_LATENCY_MS=0 \
    OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector.observability.svc.cluster.local:4318/v1/traces
EXPOSE 8000
CMD ["uvicorn","main:app","--host","0.0.0.0","--port","8000"]