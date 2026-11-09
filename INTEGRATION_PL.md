# Core 1.22.2 + Control 1.4.2 — wydanie integracyjne (2026-11-05)

## Granice wydania

Core: branch wydania jest potomkiem PR #26 (80ee1bf8b31cac24770da6348a42a7f44bf8a764), a ten zachowuje PR #25 i #24 oraz cały wcześniejszy łańcuch. Control: baza PR #2 (f90e0cb56585d8c31da7c6638fa39675e825f9d8). Bez scalania i automatycznego wdrożenia.

## Uruchomienie na istniejącej instalacji

1. Zachować spójną kopię SQL, pluginów, konfiguracji i światów; zatrzymać Paper. Podmiana JAR wymaga pełnego restartu, nie /reload.
2. Podmienić JAR Core. Wyłączyć osobne JAR-y RawCraftX, RawCraftPayments i RawCraftWebBridge; ich konfigurację pozostawić do jednorazowego importu. Nie kasować tabel ani historii Flyway.
3. Zachować istniejący plugins/Core/website/server.properties i klucz backendu. API ma bind=127.0.0.1, prywatny port i uprawnienia 600. Nie wystawiać go publicznie ani umieszczać sekretu w przeglądarce. Backend WWW korzysta z dotychczasowego kontraktu; wydanie nie zmienia strony.
4. Sprawdzić plugins/Core/website/integration.yml: most.wlaczone, platnosci.wlaczone, platnosci.tryb-testowy. Nowa instalacja domyślnie nie włącza płatności, a import zachowuje poprzedni tryb. LIVE musi być świadomą decyzją operatora po QA TEST.
5. Uruchomić Paper i sprawdzić migracje do V134 oraz status mostu. Ostrzeżenie MySQL 1050/42S01 z CREATE TABLE IF NOT EXISTS podczas adopcji rawcraft_web_topups_v1 nie jest błędem migracji. Przy niezgodnym schemacie płatności blokują się; nie usuwać tabeli, tylko ustalić niezgodność z kopii schematu.
6. Zaktualizować Control do 1.4.2 z instalatora lub aktualizatora GitHub. W profilu SSH ustawić właściwy katalog serwera Minecraft. W zakładce Płatności Core sprawdzić świeży status. Klucz odczytuje skrypt uruchomiony na VPS; nie trafia do panelu.
7. Przed LIVE wykonać autoryzowaną transakcję TEST oraz powtórzyć ten sam identyfikator: portfel nie może wzrosnąć drugi raz. Sprawdzić odrzucanie błędnego klucza/UUID/kwoty, brak zielonego statusu po rozłączeniu backendu i po restarcie. Porównać portfel i pojedynczy wpis księgi, bez ujawniania kluczy i danych graczy.

/core edycja nowa jest osobną operacją destrukcyjną wymagającą potwierdzenia z konsoli. Aktualizacja jej nie wywołuje. Szczegóły resetu i zachowywanych danych: EDITION-PAYMENT-SECURITY-2026-11-05.md. Pozostawiony world zachowuje także istniejące kontenery; reset SQL nie czyści ich zawartości.

Nie usuwać ani nie regenerować plugins/Core/storage/rtp-pad-secret.bin. Jeżeli log zgłasza uprawnienia: właściwy właściciel i chmod 600. Klucz podpisuje już istniejące przedmioty RTP.

## Weryfikacja i ograniczenia

Workflow wydania wykonuje pełne testy Java, test na odizolowanym MySQL 8.4, Python i wszystkie paczki/atlasy przed publikacją Core. Panel przechodzi Linux/Windows go vet i race, Playwright, testy skryptów SSH oraz instalację, aktualizację i deinstalację instalatora. Publikowane archiwa mają manifest SHA-256 i dokładny commit. Nie są to testy w grze ani potwierdzenie wdrożenia na VPS.
