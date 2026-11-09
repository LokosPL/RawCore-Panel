RAWCRAFT Control 1.4.4 — Windows 10/11 x64

URUCHOMIENIE
1. Pobierz RAWCRAFT-Control.exe z GitHub Releases lub artefaktów GitHub Actions.
2. Umieść EXE w katalogu, w którym Twoje konto może zapisywać pliki.
3. Uruchom aplikację. Panel otwiera się we własnym oknie Windows przez systemowy WebView2.
   Gdy brak WebView2, używa trybu aplikacji zainstalowanego Edge; nic nie instaluje.
4. Podaj IP/host VPS, port SSH, użytkownika i hasło. Zostaw zaznaczone
   zapamiętywanie hasła i automatyczne połączenie. Kolejne kliknięcie EXE
   uruchomi panel i zaloguje się zapisanym profilem bez wpisywania hasła.
   Możesz przenieść pojedynczy EXE na pulpit; nie potrzeba instalatora.

Wymagania komputera: Windows 10/11 x64, klient OpenSSH (ssh.exe), Edge.
Wymagania VPS: Linux, Bash, Python 3; PM2 dla zarządzania usługami.
Program nie instaluje agenta na VPS. Usługi muszą już istnieć w PM2.
Przy odświeżaniu danych procesy SSH i ASKPASS nie otwierają okien CMD.
EXE jest kompilowany jako aplikacja GUI z ikoną RAWCRAFT.

PANEL
- Karty usług PM2: status, CPU/RAM, start, stop, restart.
- Zasoby VPS: CPU, RAM, dysk /, czas pracy, nazwa hosta.
- Wskaźnik CPU usługi jest odnoszony do liczby rdzeni VPS; liczba pokazuje
  wartość raportowaną przez PM2 (może przekraczać 100% przy wielu rdzeniach).
- Konsola PM2 i ostatnie 256 kB logs/latest.log, pauza, odświeżanie co 2 sekundy.
- Komendy Minecraft: RCON przez tunel poleceń SSH, lokalnie na VPS.
  Ustaw enable-rcon=true, rcon.password i rcon.port w server.properties,
  następnie zrestartuj Minecraft. Port RCON nie musi być publicznie otwarty.
  Obsługiwane są proste wartości server.properties (bez ucieczek Java).
- Pliki: dwuklik nazwy otwiera folder albo edytor pliku (także klawisz Enter).
  Pozostają też przyciski Otwórz/Edytuj/Pobierz.
  Edytor nie ma limitu wielkości całego pliku: czyta fragmenty po 256 kB,
  a poprzedni/następny fragment pozwala przechodzić przez duże pliki.
  Tekst jest obsługiwany jako UTF-8 z zachowaniem BOM i CRLF/LF/CR.
  Inne kodowania i pliki binarne można edytować w trybie bajtów HEX.
  Jedna zmiana może zawierać do 8 MB nowych danych; plik może być znacznie większy.
  Zapis strumieniowo kopiuje resztę pliku, tworzy pełną kopię .rawcraft.bak
  i atomowo podmienia plik na VPS. Chroni przed nadpisaniem zmian z zewnątrz.
  Duże pliki wymagają miejsca na nowy plik i kopię na tym samym VPS.
  Ctrl+S zapisuje fragment. Do zapisu potrzebne jest prawo zapisu w katalogu.
  Zapis edytora tworzy .rawcraft.bak; upload nadpisuje plik bez kopii.
- Kopie: archiwa tar.gz katalogu serwera w .rawcraft-backups obok niego.
  Zatrzymaj serwer przed kopią świata. Kopie pozostają na tym samym VPS,
  więc pobierz je też na inny nośnik. Przywracanie wykonujesz samodzielnie.
- Terminal: pojedyncze komendy Linux z limitem czasu 45 sekund.
- Ustawienia: nazwy procesów (domyślnie minecraft i www), katalog i profil VPS.
  Hasło SSH można zapamiętać: Windows DPAPI szyfruje cały profil logowania
  dla bieżącego konta. Sekret jest związany z hostem, portem i loginem; zmiana
  profilu usuwa zapamiętane hasło. Usuń je w ustawieniach, jeśli nie jest potrzebne.
  Nie używamy hasha hasła SSH, bo do połączenia potrzebne jest oryginalne hasło.
  Hasło nie wraca do interfejsu i nie jest wpisywane do pola przez przeglądarkę.
  Ustawienia są w katalogu konfiguracji konta, niezależnie od położenia EXE.

AKTUALIZACJE GITHUB
Aplikacja sprawdza stabilne GitHub Releases przy starcie i co 5 minut.
Nowa wersja pobiera się automatycznie w tle; suma SHA-256 jest sprawdzana.
Instalacja następuje po wybraniu „Uruchom ponownie i zaktualizuj”.
Aplikacja rozłączy SSH i otworzy nowe okno. Procesy na VPS nadal działają.
Poprzedni EXE pozostaje pod nazwą .old. Błąd podmiany przywraca poprzedni plik.
Komunikaty błędów instalacji: katalog cache konta / RawcraftControl / updates.

Repozytorium aktualizacji: LokosPL/RawCore-Panel (publiczne):
- W Ustawieniach wklej fine-grained token GitHub ograniczony do tego repozytorium
  z uprawnieniem Contents: Read-only. Token tworzysz na swoim koncie GitHub.
- Windows DPAPI szyfruje zapisany token dla bieżącego użytkownika Windows.
  Aplikacja nigdy nie odsyła zapisanego tokenu do interfejsu.
- Alternatywa: zmienna środowiskowa RAWCRAFT_GITHUB_TOKEN (bez zapisu tokenu
  przez aplikację). Usunięcie tokenu w UI dotyczy konfiguracji; ustawiona zmienna
  będzie ponownie odczytana przy następnym uruchomieniu.
- Token GitHub służy tylko aktualizacjom; logowanie VPS nadal używa hasła SSH.
- Sam commit do repozytorium nie jest aktualizacją. Potrzebny jest Release
  z plikami RAWCRAFT-Control.exe i SHA256SUMS oraz tagiem vX.Y.Z.

KOMPILACJA I PUBLIKACJA
PowerShell: ./scripts/build.ps1 (wymagany Go 1.23+ i Internet).
GitHub Actions testuje Linux i Windows oraz buduje EXE z ikoną i bez konsoli.
Każdy push/PR daje artefakt RAWCRAFT-Control-Windows-x64.
Aby wydać aktualizację: zwiększ appVersion w updater.go, zatwierdź zmiany,
wykonaj git tag vX.Y.Z i git push origin vX.Y.Z albo wypchnij commit
z oznaczeniem [release] w opisie. Workflow opublikuje Release
po przejściu testów. Tag musi być zgodny z wersją aplikacji.
Sumy kontrolne chronią przed uszkodzeniem i podmianą względem Release,
ale nie zastępują podpisu Authenticode. EXE nie jest podpisany certyfikatem.

BEZPIECZEŃSTWO I OGRANICZENIA
Interfejs działa wyłącznie na 127.0.0.1, z sesją i tokenem CSRF.
OpenSSH weryfikuje known_hosts; nowy klucz jest akceptowany przy pierwszym
połączeniu (TOFU). Zmiana istniejącego klucza blokuje połączenie.
Hasło SSH jest przekazywane do lokalnego ASKPASS w środowisku procesu.
Uprawnienia plików i poleceń są takie jak uprawnienia użytkownika SSH.
Zamknięcie własnego okna Windows kończy lokalny proces i usuwa aktywną sesję
z pamięci. Przy korzystaniu z awaryjnego okna Edge użyj przycisku „Zamknij”. Działania wymagające restartu proszą o potwierdzenie.
Testy automatyczne nie zastępują próby połączenia na Twoim Windows i VPS.

ZMIANY 1.3.3 — DOSTĘP, SESJA I WYGODA
- Brak praw do pliku nie jest opisywany jako błędne hasło SSH.
- Po zalogowaniu aplikacja próbuje użyć istniejących praw sudo; można to
  przełączyć na tryb użytkownika w bieżącej sesji. Root nie potrzebuje sudo. Zakładka Dostęp i hasło VPS
  pozwala podać inne hasło sudo i włączyć/wyłączyć ten tryb w bieżącej sesji.
  Konto musi już mieć prawo wykonywania poleceń administratora przez sudo.
  Aplikacja nie omija uprawnień, nie zmienia sudoers i nie ustawia chmod 777.
- Sudo działa dla plików, edytora, kopii, RCON, latest.log i terminala.
  Procesy PM2 i ich logi pozostają na koncie SSH, żeby nie zmieniać środowiska PM2.
  Hasło sudo pozostaje tylko w pamięci sesji i nie trafia do argumentów poleceń.
- Można zmienić hasło własnego konta VPS; zmiana innych kont wymaga sudo/root.
  Nowe hasło jest potwierdzane przez system VPS/PAM, a zapamiętany profil SSH
  aktualizuje się dopiero po potwierdzeniu udanej zmiany własnego hasła.
- Okna aplikacji mają osobne ciasteczka sesji. Utracone ciasteczko można
  automatycznie odnowić tylko z istniejącego, autoryzowanego okna (nonce CSRF).
- Automatyczne błędy sprawdzania aktualizacji nie wyświetlają powiadomień nad
  panelem. Diagnostyka pozostaje w ustawieniach. Powiadomienie pojawia się
  podczas pobierania i kiedy nowa wersja jest gotowa do instalacji.
- Motywy: Czerń, Jasny i Automatycznie z systemu; wybór jest zapamiętywany.
- Konsola nie przebudowuje niezmienionych logów ani listy wyboru usługi.
  Gdy przewiniesz ją do góry lub zaznaczysz tekst, nowe dane czekają bez zmiany
  wyświetlanego fragmentu. „Pokaż najnowsze” wznawia śledzenie na dole.

Pierwsze komunikaty informacyjne sudo oraz banery SSH są oddzielane od danych
JSON aplikacji. Nie uszkadzają listy plików ani wykrywania praw administratora.
Terminal i konsola PM2 nadal pokazują wyjście błędów wykonywanych poleceń.

INSTALATOR WINDOWS (OPCJONALNY)
Pobierz RAWCRAFT-Control-Setup.exe. Wybierz folder instalacji i opcjonalny skrót
na pulpicie. Instalacja jest dla bieżącego użytkownika, bez uprawnień administratora;
domyślnie w LocalAppData/Programs/RAWCRAFT Control. Folder musi pozwalać na zapis.
Stały AppId zachowuje wybraną ścieżkę podczas ponownej instalacji.
Aktualizator pobiera RAWCRAFT-Control.exe i podmienia go dokładnie tam, skąd został
uruchomiony. Nie uruchamia instalatora, nie zmienia skrótów ani konfiguracji.
Po uruchomieniu nowej wersji aktualizuje też DisplayVersion deinstalatora.
Odinstalowanie usuwa aplikację i skróty, ale zachowuje profil i ustawienia konta.
Wersja portable pozostaje dostępna. Nie wymaga instalacji.
Kompilacja instalatora: ./scripts/build-installer.ps1 po ./scripts/build.ps1;
wymagany Inno Setup 6. Oba EXE mają sumy w SHA256SUMS.

Instalator jest budowany przy użyciu Inno Setup 6.4.3 (Jordan Russell i Martijn Laan).

ZMIANY 1.3.3
- Prawy przycisk na pliku otwiera menu Pobierz / Usuń / Edytuj.
  JAR nie ma opcji Edytuj; dwuklik nie otwiera archiwum w edytorze.
- Usunięcie wymaga potwierdzenia pełnej ścieżki. Usuwane są tylko pliki
  (lub sam link symboliczny), bez rekurencyjnego kasowania folderów.
  Zmieniony od odczytania listy plik wymaga odświeżenia przed usunięciem.
- Każde logowanie automatycznie próbuje włączyć uprawnienia administratora.
  Konto musi już mieć sudo albo być root; aplikacja nie zmienia sudoers.
  PM2 nadal pozostaje na oryginalnym koncie SSH.
- Rozpoznawanie UID używa znacznika odpornego na banery powitalne.
  Polecenia sudo nie wczytują profilu roota, żeby nie mieszać komunikatów
  profilu z danymi plików i protokołem konsoli.
- RCON zbiera odpowiedź również wtedy, gdy serwer dzieli ją na kilka pakietów.
  Komendy są wysyłane bez zamieniania nazw; błąd nieznanej komendy pochodzi
  z serwera/pluginu. Według logów RAWCRAFT status Core sprawdza się corestatus.


Wersja 1.3.4 — monitoring, dysk i wygodniejszy widok
- Zużycie dysku dla każdej usługi i największe podfoldery. Skan co 90 sekund,
  maksymalnie 6 różnych katalogów; brak dostępu jest oznaczony, nie pokazywany jako zero.
- Dzwonek z historią błędów. Monitoring co 30 sekund niezależnie od otwartej zakładki.
  Analiza ostatnich 128 KB logów, wpisów z ostatnich 15 minut i bieżącego uruchomienia.
  ERROR/FATAL/SEVERE, wyjątki i START BLOCK oznaczają błąd; zwykły WARN nie.
  Brak błędów w odczytanym fragmencie nie gwarantuje pełnej sprawności usługi.
- Status Błąd w logach nie oznacza zatrzymania procesu PM2. Szczegóły pod dzwonkiem.
- Skala Auto / 100–200% na górnym pasku; obsługa rozmiarów Full HD i 4K.
- Ekran startowy z pulsującym logo zamiast formularza przy automatycznym logowaniu.
- Aktualizacja dostępna także przed połączeniem z VPS; wykrywanie braku dostępu GitHub.
  Repozytorium jest prywatne: wymagany własny token fine-grained ograniczony do
  RawCore-Panel z Contents: Read-only. Zapisz go raz w ustawieniach aplikacji.
  Tokenu połączenia GitHub w ChatGPT nie można przenieść automatycznie do aplikacji.


Wersja 1.3.5 — kolorowa konsola i ukryty lokalny dostęp GitHub
- Ostrzeżenia: żółty; błędy i blokady startu: czerwony.
- Kolory Minecraft legacy/RGB i ANSI w logach oraz odpowiedziach RCON.
- Zaznaczenie tekstu i przewinięty podgląd pozostają stabilne.
- Dostęp do prywatnych wydań autoryzuje się raz w osobnym oknie „Dostęp do aktualizacji”.
  Zapis jest szyfrowany przez Windows DPAPI; pole jest czyszczone i nie pokazuje zapisanego tokenu.
  Token nie jest częścią źródeł ani plików EXE. Wcześniej zapisany dostęp jest zachowany.


Kompilacja TEST — automatyczny dostęp GitHub
Przy pierwszym uruchomieniu Windows automatycznie zapisuje dostarczony testowy dostęp.
Wersja zawiera współdzielony klucz możliwy do odzyskania z EXE. Przekazuj ją wyłącznie
osobom, którym zezwalasz na odczyt repozytorium. Do testów pobierz portable EXE.

Wersja 1.4.0: pełny terminal SSH (PTY, nano, cd, Ctrl+C), metryki co 3 sekundy, logi co 2 sekundy i atomowe przesyłanie hotfixów z kopią .rawcraft.bak. Uprawnienia terminala wynikają z konta SSH; sudo -i wymaga praw na VPS.

Wersja 1.4.1: panel zarządzania odświeża się automatycznie, bez przycisku Odśwież i napisów z czasem ostatniego odczytu. Dystrybucja projektu obejmuje aplikację RawCraft Control dla Windows.


PŁATNOŚCI CORE
Zakładka Płatności Core pokazuje gotowość Core, tryb TEST/LIVE i świeżość mostu WWW.
Wymaga Core z endpointem POST /v1/admin/payments/status. Połączenie przebiega
przez SSH do lokalnego API VPS; port i klucz są odczytywane na VPS z pliku
plugins/Core/website/server.properties (bind=127.0.0.1, prawa 600). Klucz nie
trafia do przeglądarki ani ustawień panelu. Ekran nie wykonuje wpłat ani resetu edycji.
Status Core nie jest potwierdzeniem płatności ani dostępności operatora PayByLink.

Wersja 1.4.3: poprawione ponowne otwieranie okna po aktualizacji (także ze starego aktualizatora SW_HIDE). Okno RAWCRAFT — Aktualizacja panelu pokazuje instalację i czeka na działający nowy panel. Zachowuje ustawienia i obsługuje cofnięcie nieudanego startu. Błędy pobierania pozostają widoczne.

Wersja 1.4.4: terminal odbiera echo SSH natychmiast po danych zamiast czekać na zegar 300 ms. Odczyt nie blokuje pisania; bufor pierścieniowy i porcje renderowania ograniczają koszt długich logów. Bez lokalnego dopisywania haseł lub fałszowania echa serwera.
