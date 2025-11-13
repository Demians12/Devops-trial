import os
import random
import time
from datetime import datetime, timedelta, time as dt_time, date as dt_date
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

PROFESSIONALS = [
    {
        "id": 2684,
        "name": "Dr(a). Pat Duarte",
        "specialty": {"id": 55, "name": "Cardiologia"},
    },
    {
        "id": 512,
        "name": "Dr. Ícaro Menezes",
        "specialty": {"id": 77, "name": "Dermatologia"},
    },
    {
        "id": 782,
        "name": "Dr(a). Helena Faria",
        "specialty": {"id": 33, "name": "Pediatria"},
    },
    {
        "id": 903,
        "name": "Dr. André Ribeiro",
        "specialty": {"id": 18, "name": "Ortopedia"},
    },
]

UNITS = [
    {"id": 901, "name": "Clínica Central", "room": {"id": 12, "name": "Sala Azul"}},
    {"id": 905, "name": "Unidade Bela Vista", "room": {"id": 203, "name": "Consultório 3"}},
    {"id": 910, "name": "Centro Norte", "room": {"id": 21, "name": "Sala Verde"}},
    {"id": 915, "name": "Hub Telemedicina", "room": {"id": 7, "name": "Estúdio 1"}},
]


def align_to_half_hour(dt: datetime) -> datetime:
    minute = dt.minute
    remainder = minute % 30
    if remainder != 0:
        dt += timedelta(minutes=30 - remainder)
    return dt.replace(second=0, microsecond=0)


def resolve_professional(professional_id: int):
    for professional in PROFESSIONALS:
        if professional["id"] == professional_id:
            return professional
    return PROFESSIONALS[0]


def resolve_unit(unit_id: int):
    for unit in UNITS:
        if unit["id"] == unit_id:
            return unit
    return UNITS[0]


def normalize_start_date(requested: dt_date | None, today: dt_date) -> dt_date:
    if requested is None:
        return today
    if requested < today:
        return today
    return requested


def build_schedule_payload(
    professional_id: int,
    unit_id: int,
    days: int = 15,
    start_date: dt_date | None = None,
):
    professional = resolve_professional(professional_id)
    unit = resolve_unit(unit_id)

    days = max(15, min(days, 30))
    now = datetime.utcnow()
    today = now.date()
    base_date = normalize_start_date(start_date, today)

    schedules = []

    for offset in range(days):
        day_date = base_date + timedelta(days=offset)
        is_today = day_date == today

        if offset == 0:
            if is_today:
                start = align_to_half_hour(now + timedelta(minutes=15))
            else:
                start = datetime.combine(day_date, dt_time(hour=8, minute=0))
        else:
            start = datetime.combine(day_date, dt_time(hour=8, minute=0))

        morning = datetime.combine(day_date, dt_time(hour=8, minute=0))
        if start < morning:
            start = morning

        slots = []
        current = start
        limit = datetime.combine(day_date, dt_time(hour=18, minute=0))
        if current > limit:
            continue

        while len(slots) < 8 and current <= limit:
            slots.append(
                {
                    "start": current.strftime("%H:%M"),
                    "available": random.random() > 0.25,
                }
            )
            current = current + timedelta(minutes=30)

        schedules.append(
            {
                "professional": {
                    "id": professional["id"],
                    "name": professional["name"],
                },
                "unit": {"id": unit["id"], "name": unit["name"]},
                "room": unit.get("room"),
                "specialty": professional["specialty"],
                "date": day_date.strftime("%Y-%m-%d"),
                "slots": slots,
            }
        )

    return schedules


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
            "/v1/appoints/available-schedule",
            "/v2/appoints/available-schedule",
        ],
    }


@app.get("/metrics")
def metrics():
    return Response(generate_latest(), media_type=CONTENT_TYPE_LATEST)


@app.get("/v1/appoints/available-schedule")
@observe_route("/v1/appoints/available-schedule")
def available_schedule(
    professional_id: int = 2684,
    unit_id: int = 901,
    days: int = 15,
    start_date: dt_date | None = None,
):
    days_requested = days
    normalized_days = max(15, min(days_requested, 30))
    today = datetime.utcnow().date()
    base_date = normalize_start_date(start_date, today)
    schedules = build_schedule_payload(
        professional_id, unit_id, normalized_days, start_date=base_date
    )
    return {
        "success": True,
        "filters": {
            "professional_id": professional_id,
            "unit_id": unit_id,
            "generated_at": datetime.utcnow().isoformat(),
            "days_requested": days_requested,
            "days_returned": normalized_days,
            "start_date_requested": start_date.isoformat() if start_date else None,
            "start_date_applied": base_date.isoformat(),
        },
        "response": schedules,
    }