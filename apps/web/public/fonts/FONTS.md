# Self-hosted fonts (OD-17)

All three families are licensed under the SIL Open Font License 1.1. Licence texts travel with the files:
`OFL-IBM-Plex.txt` (IBM Plex Sans and Mono, from `github.com/IBM/plex` `LICENSE.txt`, fetched 2026-11-18)
and `OFL-Newsreader.txt` (from `github.com/productiontype/Newsreader` `OFL.txt`, fetched 2026-11-18).
Both were checked to contain the "SIL OPEN FONT LICENSE Version 1.1" text. The woff2 files come from the
Fontsource npm packages (each package ships the upstream OFL as `LICENSE`): the Latin subset only.

| File | Source package | Version | SHA-256 |
|---|---|---|---|
| ibm-plex-sans-latin-400-normal.woff2 | @fontsource/ibm-plex-sans | 5.3.0 | 3b646991d30055a93a4ecc499713d4347953a74a947ecab435ab72070cbdab0e |
| ibm-plex-sans-latin-500-normal.woff2 | @fontsource/ibm-plex-sans | 5.3.0 | 0717336fb31fcdcde4b8deb3675bb4a0f7f6d484864afcd6751ac29975962203 |
| ibm-plex-sans-latin-600-normal.woff2 | @fontsource/ibm-plex-sans | 5.3.0 | 8960851d691c054ed38e259bdcf1a6190d157b4203ed5bb32c632a863fb8ec2f |
| ibm-plex-mono-latin-400-normal.woff2 | @fontsource/ibm-plex-mono | 5.3.0 | 08949f728dc52d528e69b1667d15c89a5686a4ee9a296ff90983985f99c380f7 |
| ibm-plex-mono-latin-500-normal.woff2 | @fontsource/ibm-plex-mono | 5.3.0 | 01d285447409c8a588692162439a038b8cbd7871309ee20267b0d2d91c6e8e22 |
| newsreader-latin-wght-normal.woff2 | @fontsource-variable/newsreader | 5.3.0 | 62981321d9a3cc7a61a73792729043703fd6112da86e8ec848bb57f088578757 |
| newsreader-latin-wght-italic.woff2 | @fontsource-variable/newsreader | 5.3.0 | 48bc8861b9b2ca9300747cad4fd6a3b4ac3028d364df00bd1b72097baa75e509 |

Not shipped: Latin-extended and other scripts (English-only 1.0). Metric-matched fallbacks are not measured;
CLS to be measured at M7.
