# RAWCRAFT Control 1.5.3 — metryki Paper/systemd

- Pełniejsza karta RawCore Survival: rzeczywiste CPU JVM (delta czasu procesora), RAM systemd, rozmiar folderu /opt/rawcore-survival, PID, czas pracy JVM i tryb.
- CPU pokazuje obciążenie w % jednego rdzenia; może przekroczyć 100% na procesorach wielordzeniowych.
- Skan katalogu serwera jest buforowany przez 60 sekund, aby nie obciążać dysku VPS co 3 sekundy.
- W razie braku dostępu do pomiaru wyświetla „Brak pomiaru” zamiast fałszywego zera.
- Start/Stop/Restart oraz odczyt logów systemd pozostają bez zmian. Bez modyfikowania plików Minecrafta, WWW i bazy.
