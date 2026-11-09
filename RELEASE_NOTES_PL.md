# RAWCRAFT Control 1.5.7 — menedżer plików

- Foldery można pobierać jako ZIP, a pliki binarne i JAR do komputera bez opuszczania panelu.
- Nowy mechanizm pobierania obsługuje strumień i okno zapisu (gdy system je udostępnia), z awaryjną obsługą małych plików.
- Błędy pobierania (brak pliku, uprawnień lub problem z SSH) zgłaszane są zamiast pustego pobranego pliku.
- Foldery wraz z zawartością można usuwać po wyraźnym potwierdzeniu. Zabezpieczono katalog główny instancji oraz symlinki.
- Pobieranie i usuwanie pozostają ograniczone do wybranej instancji; WWW, PM2 i MySQL nie są modyfikowane.
