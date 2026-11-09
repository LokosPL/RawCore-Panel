# RAWCRAFT Control 1.5.5 — poprawka pobierania Paper

- Naprawiono kreator: PaperMC Fill v3 zwraca URL pliku JAR w domenie `fill-data.papermc.io`, której instalator 1.5.4 nie dopuszczał.
- Dopuszczamy ten dokładny host przez HTTPS zarówno dla adresu pliku, jak i przekierowań. Pozostałe nieznane hosty i niebezpieczne URL nadal są blokowane.
- Komunikat błędu pobierania pokazuje teraz odrzucony host, aby ułatwić diagnozę.
- Zachowano weryfikację SHA-256/SHA-512, limit wielkości JAR, atomowe przygotowanie nowego katalogu, sprawdzanie portów i osobną akceptację EULA.
- Zmiana dotyczy tylko pobierania nowych instalacji. Nie modyfikuje działającego serwera ani strony WWW.
