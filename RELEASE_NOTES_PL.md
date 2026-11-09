# RAWCRAFT Control 1.5.2 — RawCore Survival w panelu

- Wykrywanie istniejącej usługi `rawcore-survival.service` na VPS przez `systemctl show`, bez instalowania drugiej kopii Minecrafta.
- Osobna karta **RawCore Survival** na pulpicie, status online/offline, PID i pamięć RAM, gdy raportuje ją systemd.
- Bezpieczne, jawne akcje Start, Stop i Restart dla jednej konkretnej usługi, z potwierdzeniem działań przerywających rozgrywkę.
- Podgląd ostatnich wpisów `journalctl` w oknie aplikacji.
- Bez zmian w Nginx, PM2, MySQL, plikach Minecrafta i stronie WWW. Usługa pozostaje zarządzana przez systemd.
- W razie braku sudo operacje administracyjne zgłaszają błąd uprawnień zamiast uruchamiać drugą instancję.
