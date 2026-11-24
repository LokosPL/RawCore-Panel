# RAWCRAFT Control 1.7.0 — przejrzysty panel i szybkie akcje

- **Żywy stan serwera na starcie**: zamiast statycznego banera panel od razu mówi, czy „Wszystko działa”, czy któraś usługa wymaga uwagi — z listą zatrzymanych usług, pierścieniem „online 2/2” oraz pigułkami CPU / RAM / dysk (zielone, żółte od 75%, czerwone od 90%).
- **Szybkie akcje Ctrl+K**: przycisk „Szukaj…” w górnym pasku i skrót Ctrl+K otwierają wyszukiwarkę sekcji, usług, instancji i akcji (nowa instancja, sprawdzenie aktualizacji, powiadomienia, zmiana motywu). Działa strzałkami i Enterem, ignoruje polskie znaki. W terminalu SSH Ctrl+K nadal trafia do powłoki.
- **Wykrywanie serwera Minecraft pod inną nazwą**: gdy proces z ustawień (np. „minecraft”) nie istnieje, a w PM2 działa podobny (np. „minecraft-survival” na Javie), pulpit pisze „Nie widzę serwera Minecraft” zamiast „Wszystko działa” i proponuje przycisk „Użyj …”, który wpisuje nazwę procesu i jego katalog do ustawień.
- **Serwer Minecraft na systemd jest widoczny w „Zarządzaniu usługą”**: gdy nie ma procesu PM2 o nazwie z ustawień, ale działa RawCore Survival (rawcore-survival.service), kliknięcie „Serwer Minecraft” pokazuje jego kartę ze startem, restartem, zatrzymaniem i logami zamiast komunikatu „Nie znaleziono”. Gdy nie ma żadnego z nich, panel wypisuje procesy PM2 znalezione na VPS i prowadzi do ustawień.
- **Kropki stanu w menu**: przy „Serwer Minecraft” i „Strona WWW” widać zieloną lub czerwoną kropkę bez wchodzenia na pulpit.
- **Spójne ikony**: wszystkie pozycje menu, kafelki statystyk, skróty i przyciski mają jeden zestaw liniowych ikon zamiast znaków tekstowych, które w części czcionek Windows wyświetlały się jako „‹”.
- **Czytelniejsza instancja**: aktywna instancja ma złoty akcent i etykietę „Aktywna”, a przycisk „Wybierz” nie dubluje już wybranej.
- **Telefon / wąskie okno**: pasek instancji nie wychodzi poza ekran, karta instancji układa się pionowo, pierścień stanu zostaje widoczny.
- Krótsze nazwy motywów (Motyw systemowy / Ciemny / Jasny), równe rozmieszczenie wskaźników w karcie Parametry VPS.
- Archiwum źródeł w wydaniu ma teraz numer bieżącej wersji (wcześniej zawsze „1.5.0”).
- Bez zmian w logice SSH, PM2, plikach, kopiach, płatnościach i aktualizatorze.

Poprzednia wersja (1.6.0): odświeżony wygląd panelu — spokojniejsze powierzchnie, czytelniejsze kafelki i karty usług.
