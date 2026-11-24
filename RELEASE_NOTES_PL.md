# RAWCRAFT Control 1.7.1 — przejrzysty panel, samonaprawa ustawień i aktualizacje bez tokenu

**Aktualizacje na bieżąco**
- Aplikacja sprawdza nowe wersje w publicznych wydaniach GitHub bez tokenu — co 5 minut i przy starcie, pobiera je w tle i weryfikuje SHA-256. Komunikat „Wymagana jednorazowa autoryzacja…” znika. Token jest potrzebny tylko, gdyby repozytorium stało się prywatne.

**Serwer Minecraft zawsze widoczny**
- Gdy w ustawieniach jest zła nazwa procesu (np. „minecraft”, a w PM2 działa „minecraft-survival”), panel to wykrywa i pokazuje pasek z przyciskiem „Użyj …”, który jednym kliknięciem zapisuje poprawną nazwę i katalog.
- Gdy katalog serwera w ustawieniach różni się od katalogu, w którym naprawdę działa proces, pasek „Katalog serwera Minecraft jest nieaktualny” poprawia go jednym kliknięciem (to usuwa błędy „Katalog procesu nie odpowiada instancji” w logach i „du: cannot access” przy pomiarze dysku).
- Serwer na systemd (rawcore-survival.service) jest widoczny po kliknięciu „Serwer Minecraft”.
- Pulpit nie pisze już „Wszystko działa”, gdy serwera Minecraft nie widać.

**Instalator i odinstalowanie**
- Odinstalowanie najpierw zamyka działający panel, a potem pyta, czy usunąć wszystko: ustawienia, profile VPS, zapamiętane hasło SSH, token, historię powiadomień, dane okna i pobrane aktualizacje. „Tak” czyści komputer do zera; „Nie” zachowuje ustawienia na wypadek ponownej instalacji. Pliki na VPS nie są ruszane.
- Usuwane są też resztki po aktualizacjach (.old/.new), skrót na pulpicie, wpis w „Aplikacjach” i pusty folder programu.
- Skrót na pulpicie jest tworzony domyślnie; instalator i wpis w Windows mają pełną nazwę, wersję i link do projektu.
- Tryb skryptowy: `unins000.exe /VERYSILENT /PURGE` usuwa wszystko bez pytań.

**Wygląd i wygoda**
- Pulpit pokazuje stan na żywo: „Wszystko działa” / „N usług wymaga uwagi”, pierścień usług online i pigułki CPU / RAM / dysk (żółte od 75%, czerwone od 90%).
- Szybkie akcje Ctrl+K (albo „Szukaj…”): sekcje, usługi, instancje, nowa instancja, aktualizacje, motyw. W terminalu SSH Ctrl+K zostaje dla powłoki.
- Kropki stanu przy „Serwer Minecraft” i „Strona WWW” w menu.
- Jeden zestaw liniowych ikon w całym panelu, wyraźnie oznaczona aktywna instancja, poprawiony układ na wąskich ekranach, krótsze nazwy motywów.

**Technicznie**
- Testy UI w GitHub Actions przypięte do Ubuntu 24.04 (nowy ubuntu-latest blokował każde wydanie).
- Archiwum źródeł w wydaniu nosi numer bieżącej wersji.
- Bez zmian w obsłudze SSH, PM2, plików, kopii i płatności.

Poprzednia wersja (1.6.0): odświeżony wygląd panelu — spokojniejsze powierzchnie, czytelniejsze kafelki i karty usług.
