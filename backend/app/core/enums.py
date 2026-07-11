import enum


class Role(str, enum.Enum):
    guru = "guru"            # Гуру (Махарадж) — видит всех
    secretary = "secretary"  # Секретарь/помощник — ведёт данные, отчёты
    curator = "curator"      # Куратор/наставник — видит закреплённых
    student = "student"      # Ученик — своя анкета


class InitiationStatus(str, enum.Enum):
    aspirant = "aspirant"        # аспирант
    pranama = "pranama"          # получил пранама-мантру
    recommended = "recommended"  # рекомендован
    harinama = "harinama"        # первая инициация (харинама)
    brahman = "brahman"          # вторая инициация (брахман)


class ThreadKind(str, enum.Enum):
    question = "question"  # приватный вопрос гуру (ученик ↔ гуру)
    report = "report"      # ежемесячный отчёт о служении (ученик, наставник, гуру)
    approval = "approval"  # чат зарегистрированного кандидата с апрувером до апрува


class MaritalStatus(str, enum.Enum):
    single = "single"
    married = "married"
    brahmachari = "brahmachari"
    sannyasi = "sannyasi"
    widowed = "widowed"
    other = "other"
