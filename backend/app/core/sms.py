"""Отправка SMS через SMSC.ru (тот же провайдер, что в проекте bid)."""
import json
import logging
import urllib.parse
import urllib.request

from app.core.config import settings

logger = logging.getLogger("sms")
_SMSC_URL = "https://smsc.ru/sys/send.php"


def normalize_phone(raw: str) -> str:
    """Привести телефон к формату 7XXXXXXXXXX (11 цифр, РФ)."""
    digits = "".join(ch for ch in (raw or "") if ch.isdigit())
    if len(digits) == 11 and digits[0] == "8":
        digits = "7" + digits[1:]
    if len(digits) == 10:
        digits = "7" + digits
    return digits


def send_sms(phone: str, message: str) -> bool:
    """Отправить SMS. В dev (SMSC_ENABLED=False) — только лог, без реальной отправки."""
    if not settings.SMSC_ENABLED or not settings.SMSC_LOGIN:
        logger.info("SMS mock → %s: %s", phone, message)
        return True
    params = urllib.parse.urlencode({
        "login": settings.SMSC_LOGIN,
        "psw": settings.SMSC_PASSWORD,
        "phones": phone,
        "mes": message,
        "fmt": 3,  # JSON
    })
    try:
        with urllib.request.urlopen(f"{_SMSC_URL}?{params}", timeout=10) as resp:
            data = json.loads(resp.read().decode("utf-8", "replace"))
        if isinstance(data, dict) and data.get("error"):
            logger.error("SMSC error: %s", data)
            return False
        return True
    except Exception as e:  # noqa: BLE001
        logger.error("SMS send failed: %s", e)
        return False
