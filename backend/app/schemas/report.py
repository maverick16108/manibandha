from pydantic import BaseModel


class CountByKey(BaseModel):
    key: str
    count: int


class ReportSummary(BaseModel):
    total: int
    by_status: list[CountByKey]
    by_country: list[CountByKey]
    by_temple: list[CountByKey]
    ready_for_initiation: int
