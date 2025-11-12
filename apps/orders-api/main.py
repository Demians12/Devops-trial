import os, random, time
from fastapi import FastAPI, Response
from prometheus_client import Counter, Histogram, generate_latest, CONTENT_TYPE_LATEST
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.sdk.resources import Resource

ERROR_RATE = float(os.getenv("ERROR_RATE", "0.01"))
EXTRA_LATENCY_MS = int(os.getenv("EXTRA_LATENCY_MS", "0"))
SERVICE_NAME = os.getenv("SERVICE_NAME", "orders-api")

resource = Resource(attributes={"service.name": SERVICE_NAME})
provider = TracerProvider(resource=resource)
processor = BatchSpanProcessor(OTLPSpanExporter(endpoint=os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT","http://otel-collector.observability.svc.cluster.local:4318/v1/traces")))
provider.add_span_processor(processor)
trace.set_tracer_provider(provider)

app = FastAPI(title="orders-api")
FastAPIInstrumentor.instrument_app(app)

REQS = Counter("http_requests_total", "Total HTTP requests", ["route","status"])
LAT = Histogram("http_request_duration_seconds", "Request latency", buckets=(0.05,0.1,0.2,0.3,0.5,0.75,1,2,5))

@app.get("/healthz")
def health():
    return {"status": "ok"}

@app.get("/")
def root():
    return {"service": SERVICE_NAME, "status": "ok", "endpoints": ["/healthz", "/api/v1/orders", "/api/v2/orders"]}

@app.get("/metrics")
def metrics():
    return Response(generate_latest(), media_type=CONTENT_TYPE_LATEST)

@app.get("/api/v1/orders")
def v1_orders():
    with LAT.time():
        if EXTRA_LATENCY_MS > 0:
            time.sleep(EXTRA_LATENCY_MS/1000)
        status = 200
        if random.random() < ERROR_RATE:
            status = 500
        REQS.labels(route="/api/v1/orders", status=str(status)).inc()
        if status >= 500:
            return Response("error", status_code=status)
        return {"version":"v1","orders":[{"id":1},{"id":2}]}

@app.get("/api/v2/orders")
def v2_orders():
    with LAT.time():
        if EXTRA_LATENCY_MS > 0:
            time.sleep(EXTRA_LATENCY_MS/1000)
        status = 200
        if random.random() < ERROR_RATE:
            status = 500
        REQS.labels(route="/api/v2/orders", status=str(status)).inc()
        if status >= 500:
            return Response("error", status_code=status)
        return {"version":"v2","orders":[{"id":1},{"id":2},{"id":3}]}