RAWCRAFT Control 1.4.9 — wiele instancji i kreator

Baza: 6402aeeaf2ed61adc91d219fa38b119df82f769c / PR #9 (1.4.8), zachowująca PR #8 / 1.4.7. Bez scalania i instalacji na VPS.

Dodano listę instancji z osobnym katalogiem, procesem PM2, portami, wersją, Java i RAM. Wcześniejszy profil pozostaje serwerem głównym. Wybór serwera wiąże pliki, RCON, logi, backupy i status płatności z jego konfiguracją. Każde żądanie ma identyfikator instancji i rewizję profilu; błędne ścieżki i stare rewizje są odrzucane, numery PM2 są dodatkowo weryfikowane po nazwie i katalogu procesu. Terminal otwiera się w katalogu instancji i zachowuje własny identyfikator sesji.

Przełączanie czyści dane poprzedniej instancji i ignoruje spóźnione odpowiedzi. Nie pozwala przełączyć celu w trakcie operacji zapisu/przesyłania/akcji/edycji.

Kreator: najpierw podgląd sprawdza katalog, konflikty portów TCP IPv4/IPv6 oraz porty zapisane w server.properties istniejących instancji, także zatrzymanych, nazwy i katalogi PM2 oraz działanie Java. Po świadomym wybraniu „Przygotuj pliki” tworzy wyłącznie nowy katalog, kopiuje wskazany JAR silnika i zapisuje osobne server.properties i lokalne API Core na osobnym porcie (nowy klucz API, most i płatności wyłączone), niezaakceptowaną EULA, nieaktywny Transfer/Voting i izolowaną bazę Core SQLite. Nie kopiuje danych, sekretów, produkcyjnej bazy, ekonomii ani konfiguracji Discord/płatności; Core instalujesz z wydania osobno. Zapisuje ecosystem.config.json z autorestart=false, ale nie rejestruje i nie uruchamia procesu. Przed startem operator weryfikuje JAR/Java, połączenia i firewall oraz akceptuje EULA.

Jest to przygotowanie instancji, nie automatyczna instalacja na VPS użytkownika. Nie zweryfikowano rzeczywistej topologii/portów VPS ani Minecraft na żywo. Natywne transfery Core wymagają trzech uruchomionych instancji (głównej, testowej i lobby); konfiguracja nie zakłada proxy.

Zachowano aktualizator z potwierdzeniem startu i rollbackiem, małe okno błędu, odporność na zablokowane .old/.new, stary helper, terminal z prawdziwym echo i ochroną poprzedniej sesji, host key oraz klucz płatności pozostający na VPS. Testy: Go vet/race Linux/Windows, Playwright, Python i rzeczywiste EXE/instalator w Actions. Wynik bieżącego wydania należy sprawdzić w CI.
