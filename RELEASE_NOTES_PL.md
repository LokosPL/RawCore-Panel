# RAWCRAFT Control 1.5.6 — poprawka fałszywych alarmów Minecraft

- Naprawiono monitor logów PM2: starym wpisom bez daty nie jest przypisywana data późniejszego dopisania do całego pliku.
- Gdy log zawiera datowane komunikaty, niedatowane kontynuacje otrzymują czas poprzedniego wpisu, a wpisy sprzed pierwszej daty nie wywołują alarmu.
- Logi zachowują pełną historię, niczego nie czyścimy i nie restartujemy Minecrafta ani WWW.
- Zachowana detekcja aktualnych ERROR/FATAL/Exception, dodane testy regresji dla starego błędu read-only z nowymi ostrzeżeniami Java.
