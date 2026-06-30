"""AI biometric service.

Scaffold: a FastAPI health endpoint. Live payment auth runs on-device (ADR-0006), so
this service is off the critical path. Optional future roles: enrollment liveness /
quality checks, or a local-network fallback match (docs/plans/mobile-app.md §5).
"""

from fastapi import FastAPI

app = FastAPI(title="ai-biometric", version="0.0.0")


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok", "service": "ai-biometric"}
