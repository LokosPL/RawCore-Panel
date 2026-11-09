# RAWCRAFT Control 1.5.0 — terminal SSH i nowy panel zarządzania

## Terminal SSH
- Nowy edytor wieloliniowy do wklejania serii komend (do 128 KiB UTF-8).
- Potwierdzenie przed uruchomieniem skryptu, podgląd liczby linii i rozmiaru, skrót Ctrl+Enter.
- Skrypt działa w osobnej, nieinteraktywnej sesji SSH przez `bash -se`: polecenia są wykonywane w kolejności, a seria zatrzymuje się po błędzie.
- Zbiorczy wynik standardowego wyjścia i błędów, kod zakończenia, ograniczenie do 512 KiB i możliwość kopiowania.
- Interaktywny terminal xterm nadal obsługuje zwykłe polecenia, historię, edytory, Ctrl+C i wklejanie przez Ctrl+V.
- Limity, walidacja UTF-8, ochrona przed znakami sterującymi i ograniczenie wykonania do 120 sekund.

## Interfejs
- Nowy pulpit operacyjny z wyraźnymi skrótami do terminala i plików, odświeżona nawigacja, karty usług oraz widoki jasny i ciemny.
- Terminal w oddzielnej konsoli o wysokim kontraście, status sesji oraz wygodny, responsywny edytor skryptów.
- Uporządkowane odnośniki aktualizacji do rzeczywistego repozytorium `LokosPL/RawCore-Panel`.

## Uwagi bezpieczeństwa
- Skrypty wykonują rzeczywiste operacje na VPS i mogą usuwać dane. Używaj ich świadomie.
- Skrypty są odpalane w osobnej powłoce Bash z katalogiem wybranej instancji. Nie dziedziczą zmian `cd` ani `sudo -i` z terminala interaktywnego.
- Do programów interaktywnych i podawania hasła `sudo` użyj górnego terminala.
- Uwierzytelnianie i weryfikacja klucza hosta SSH pozostają bez zmian.

## Wymagania
Windows x64, aktualne WebView2 i dostęp SSH do VPS. Aktualizacja dostępna w panelu lub przez instalator na GitHub Releases.
