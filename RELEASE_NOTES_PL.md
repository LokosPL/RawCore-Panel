# RAWCRAFT Control 1.5.4 — kreator Paper i pluginy

- Nowy kreator Paper 26.3 z wyborem ViaVersion, ViaBackwards, ViaRewind i WorldEdit.
- Automatyczne pobieranie wyłącznie stabilnych wersji kompatybilnych z Paper 26.3 przez oficjalne API PaperMC i Modrinth; sprawdzanie sum SHA-512/SHA-256.
- Weryfikacja Java 25, portów, katalogów oraz kolizji procesów; pobieranie do osobnego katalogu tymczasowego bez nadpisywania istniejących światów.
- Osobny krok „Akceptuję EULA i uruchamiam w PM2”. Panel nie uruchamia automatycznie niezaakceptowanych serwerów.
- Domyślnie port 25566, aby nowa instancja nie kolidowała z dotychczasowym Paper na 25565.
- WWW pozostaje na pulpicie także po przełączeniu instancji Minecraft.
- Nie usuwamy starego Paper, świata ani bazy MySQL; przełączenie i usuwanie starego serwera dopiero po weryfikacji nowego.
