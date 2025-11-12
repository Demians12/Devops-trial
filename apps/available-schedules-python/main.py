import os
import random
import time
from datetime import datetime, timedelta
from functools import wraps

from fastapi import FastAPI, Response
from prometheus_client import CONTENT_TYPE_LATEST, Counter, Histogram, generate_latest
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

ERROR_RATE = float(os.getenv("ERROR_RATE", "0.01"))
EXTRA_LATENCY_MS = int(os.getenv("EXTRA_LATENCY_MS", "0"))
SERVICE_NAME = os.getenv("SERVICE_NAME", "available-schedules")

OTEL_ENDPOINT = os.getenv(
    "OTEL_EXPORTER_OTLP_ENDPOINT",
    "http://otel-collector.observability.svc.cluster.local:4318/v1/traces",
)

resource = Resource.create({"service.name": SERVICE_NAME})
provider = TracerProvider(resource=resource)
processor = BatchSpanProcessor(OTLPSpanExporter(endpoint=OTEL_ENDPOINT))
provider.add_span_processor(processor)
trace.set_tracer_provider(provider)
tracer = trace.get_tracer(__name__)

app = FastAPI(title="available-schedules-python")
FastAPIInstrumentor.instrument_app(app)

REQS = Counter("http_requests_total", "Total HTTP requests", ["route", "status"])
LAT = Histogram(
    "http_request_duration_seconds",
    "Request latency",
    buckets=(0.05, 0.1, 0.2, 0.3, 0.5, 0.75, 1, 2, 5),
)


def build_available_schedule(professional_id: int, unit_id: int, days: int = 3):
    base = datetime.utcnow()
    slots = []
    for offset in range(days):
        date = base + timedelta(days=offset)
        available_hours = [
            {"start": (date + timedelta(hours=9, minutes=30 * i)).strftime("%H:%M")}
            for i in range(0, 6)
        ]
        slots.append(
            {
                "date": date.strftime("%Y-%m-%d"),
                "hours": [hour["start"] for hour in available_hours],
            }
        )
    return {
        "professional": {"id": professional_id, "name": "Dr(a). Pat Duarte"},
        "unit": {"id": unit_id, "name": "Clínica Central"},
        "room": {"id": 12, "name": "Sala Azul"},
        "specialty": {"id": 55, "name": "Cardiologia"},
        "schedules": slots,
    }


def observe_route(route: str):
    def decorator(handler):
        @wraps(handler)
        def wrapper(*args, **kwargs):
            with LAT.time():
                if EXTRA_LATENCY_MS > 0:
                    time.sleep(EXTRA_LATENCY_MS / 1000)
                status = 200
                with tracer.start_as_current_span(route) as span:
                    if random.random() < ERROR_RATE:
                        status = 500
                        span.set_attribute("error.type", "simulated_failure")
                        result = Response(
                            content="transient error retrieving schedule",
                            status_code=status,
                        )
                    else:
                        result = handler(*args, **kwargs)
                        if isinstance(result, Response):
                            status = result.status_code
                        else:
                            status = 200
                REQS.labels(route=route, status=str(status)).inc()
                return result

        return wrapper

    return decorator


@app.get("/healthz")
def health():
    return {"status": "ok"}


@app.get("/")
def root():
    return {
        "service": SERVICE_NAME,
        "status": "ok",
        "endpoints": [
            "/healthz",
            "/metrics",
            "/appoints/available-schedule",
            "/go/appoints/available-schedule",
        ],
    }


@app.get("/metrics")
def metrics():
    return Response(generate_latest(), media_type=CONTENT_TYPE_LATEST)


@app.get("/appoints/available-schedule")
@observe_route("/appoints/available-schedule")
def available_schedule(professional_id: int = 2684, unit_id: int = 901):
    payload = {
        "success": True,
        "filters": {
            "professional_id": professional_id,
            "unit_id": unit_id,
            "generated_at": datetime.utcnow().isoformat(),
        },
        "response": [build_available_schedule(professional_id, unit_id)],
    }
    return payload


@app.get("/python/appoints/available-schedule")
def available_schedule_prefixed(professional_id: int = 2684, unit_id: int = 901):
    return available_schedule(professional_id=professional_id, unit_id=unit_id)