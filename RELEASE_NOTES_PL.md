# RAWCRAFT Control 1.5.9 — naprawa błędu podczas przesyłania folderów

- Każdy plik przesyłany jako część folderu ma weryfikowany i w razie potrzeby odtwarzany katalog nadrzędny tuż przed zapisem na VPS, dzięki czemu wcześniejsze usunięcie pustego folderu nie przerywa transferu FileNotFoundError.
- Odtwarzanie katalogu działa wyłącznie wewnątrz wybranej instancji, bez podążania za dowiązaniami symbolicznymi i bez rozszerzania uprawnień.
- Zapobiegamy przypadkowemu zagnieżdżeniu folderu world/world, plugins/plugins itd. przez wskazanie właściwego katalogu nadrzędnego.
- Wyświetlamy ostrzeżenie o zatrzymaniu Minecrafta przed kopiowaniem aktywnego świata.
- Testy regresji dla odtworzenia usuniętych folderów, zabezpieczeń ścieżek i symlinków.
- Nie ingerujemy w istniejący świat, usługę PM2, WWW ani MySQL.
