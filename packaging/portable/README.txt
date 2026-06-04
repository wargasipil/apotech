Apotech - Portable Edition (SQLite)
===================================

A self-contained copy of the Apotech pharmacy app. No installer, no database
server, no administrator rights. Everything lives in this folder.

QUICK START
-----------
1. Unzip this folder anywhere you like - a normal folder, the Desktop, or a USB
   stick. (Avoid C:\Program Files; that location needs admin rights to write.)
2. Double-click "Apotech.bat".
3. The FIRST time, it asks you to create the owner login (email + password).
4. Your web browser opens to http://127.0.0.1:8080 - log in with what you set.
5. To stop the app, close the black console window.

WHERE IS MY DATA?
-----------------
Everything is inside the "data" folder next to Apotech.bat:
  data\config.yaml   - settings (port, owner login on first run, etc.)
  data\apotech.db     - the database (all your pharmacy data)
  data\backups\       - backups you create from Settings inside the app

To MOVE or COPY your installation (e.g. to another PC or a USB stick), copy the
WHOLE folder. The data travels with it.

PASSWORDS
---------
After the first run, change the owner password and add staff users from inside
the app (Users / Settings). Those changes are saved and survive restarts.

To completely RESET to a blank install, close the app and delete the "data"
folder; the next launch starts fresh and prompts for a new owner login.

NOTES
-----
- The app uses port 8080. If another program already uses it, the console shows
  an error - close that program (or this is the wrong PC for it) and retry.
- A modern browser (Chrome, Edge, Firefox) is required.
- Back up regularly: open Settings inside the app and click Create Backup, or
  just copy the "data" folder while the app is closed.
