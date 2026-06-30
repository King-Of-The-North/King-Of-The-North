from app.main import healthz


def test_healthz():
    assert healthz() == {"status": "ok", "service": "ai-biometric"}
