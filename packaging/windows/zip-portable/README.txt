================================================================
 Apotech Portable
 Pharmacy management - portable Windows distribution (SQLite)
================================================================

Two languages below: English first, then Bahasa Indonesia.
Dua bahasa di bawah: Inggris dulu, kemudian Bahasa Indonesia.


================================================================
 ENGLISH
================================================================

What is this?
-------------
Apotech Portable is a single-PC pharmacy management application
bundled into a self-contained Windows executable. It needs no
external database, no Windows service, no admin rights, and no
firewall changes. All your data lives in a single SQLite file
inside this folder.

Files in this folder:
  * apotech-portable.exe   -- the application
  * first-run.bat          -- one-time setup (creates config.yaml)
  * README.txt             -- this file

After first run the application creates two more folders:
  * data\                  -- holds apotech.db (your database)
  * backups\               -- holds per-timestamp backup folders


Quick start (3 steps)
---------------------

1. Double-click "first-run.bat".
   Enter the OWNER email and password (min 8 characters).
   The script writes config.yaml and exits.

2. Double-click "apotech-portable.exe".
   A console window opens. After a few seconds your default
   browser opens to http://127.0.0.1:8080

3. Log in with the email and password you entered in step 1.
   Change the password from Users -> Edit if you wish.

That's it. The application is now running.


Daily operations
----------------

* Start the app: double-click apotech-portable.exe.
* Stop the app:  close the console window (the one running the
                 EXE; the browser tab can stay open until then).
* Where is my data? In the data\ folder next to the EXE.
                 Back up the whole folder and you back up
                 everything.
* Multiple users at once? Apotech Portable is single-PC,
                 single-writer (SQLite serializes writes).
                 Multiple cashiers on multiple PCs need the
                 Postgres-backed "full" installer instead.


Backup
------

Three ways - they all produce the same backup_<timestamp>\ folder
inside backups\ :

a) From the web UI:  log in as OWNER -> Settings -> Backups
                     -> Create. The new entry appears at the top.

b) From the command line:  open Command Prompt in this folder
                           (Shift + right-click the folder, then
                           "Open command window here") and type:
                              apotech-portable.exe --backup

c) Schedule a nightly backup:  Windows Task Scheduler -> Create
                               Basic Task -> point it at
                               apotech-portable.exe with the
                               argument  --backup .

Each backup is a single SQLite snapshot you can open in
"DB Browser for SQLite" (free download) for spot-checking.


Restore
-------

1. Close the running EXE.
2. Copy   backups\backup_<TIMESTAMP>\database.db
   over   data\apotech.db
3. Re-open apotech-portable.exe.

That's the entire procedure. No special tools needed.


Moving the install to another PC (or to a USB stick)
----------------------------------------------------

Just copy the entire folder (apotech-portable.exe, config.yaml,
data\, backups\) to the new location. Double-click the EXE
there - everything works the same. Your owner credentials, your
JWT secret, and your data all live in this folder.

Tip: avoid running directly from a USB stick during heavy use.
SQLite is happy on SSD/HDD storage; cheap USB sticks can be
slow. Copy to the local disk for normal use, sync back to USB
for offsite backup.


FAQ
---

Q: Do I need internet?
A: No. The app runs entirely on this PC.

Q: Can other people on my office network reach it?
A: No, by default. The EXE binds 127.0.0.1 (this PC only).
   That keeps it secure for the portable use case. If you need
   LAN access, use the full "Apotech" installer instead.

Q: What if port 8080 is busy?
A: Open config.yaml in Notepad, change server.port to a free
   number (try 8090), save, and relaunch the EXE.

Q: I forgot the owner password.
A: Delete config.yaml AND the data\ folder, then run
   first-run.bat again. WARNING: this wipes your data. If you
   have a backup, restore it before logging back in. If your
   data is not yet backed up, contact the developer for help
   resetting the password in-place.

Q: Where are the logs?
A: They print to the console window the EXE is running in.
   Save them by redirecting:
     apotech-portable.exe > apotech.log 2>&1


Troubleshooting
---------------

* The EXE opens then closes immediately:
  open Command Prompt in this folder and type:
     apotech-portable.exe
  to see the error message before the window disappears.

* "config.yaml not found":
  you skipped step 1. Double-click first-run.bat to create it.

* "address already in use":
  another program (or another copy of Apotech Portable) is
  using port 8080. Change server.port in config.yaml or close
  the other program.

* Browser does not open:
  ignore it - just open http://127.0.0.1:8080 manually in any
  browser.


Need more detail?
-----------------

See DEPLOYMENT.md in the source repository, or the developer's
README at packaging/windows/README-portable.md .


================================================================
 BAHASA INDONESIA
================================================================

Apa ini?
--------
Apotech Portable adalah aplikasi manajemen apotek satu-PC yang
dikemas dalam satu file EXE Windows. Tidak butuh database
eksternal, tidak butuh service Windows, tidak butuh hak admin,
tidak butuh perubahan firewall. Semua data Anda berada dalam
satu file SQLite di dalam folder ini.

File di folder ini:
  * apotech-portable.exe   -- aplikasi utama
  * first-run.bat          -- setup sekali pakai (buat config.yaml)
  * README.txt             -- file ini

Setelah dijalankan pertama kali, aplikasi membuat dua folder:
  * data\                  -- berisi apotech.db (database Anda)
  * backups\               -- berisi folder backup per-timestamp


Cara mulai (3 langkah)
----------------------

1. Klik dua kali "first-run.bat".
   Masukkan email PEMILIK dan password (minimal 8 karakter).
   Skrip akan menulis config.yaml lalu selesai.

2. Klik dua kali "apotech-portable.exe".
   Jendela konsol akan terbuka. Setelah beberapa detik browser
   default Anda akan terbuka ke http://127.0.0.1:8080

3. Login dengan email dan password yang Anda masukkan di
   langkah 1. Ganti password dari menu Users -> Edit jika mau.

Selesai. Aplikasi sudah berjalan.


Operasi harian
--------------

* Menjalankan: klik dua kali apotech-portable.exe.
* Menghentikan: tutup jendela konsol (yang menjalankan EXE;
                tab browser boleh tetap terbuka).
* Di mana data saya? Di folder data\ di samping EXE.
                Backup seluruh folder = backup semua data.
* Bisa multi-user? Apotech Portable adalah single-PC dan
                single-writer (SQLite mengurutkan tulisan).
                Untuk banyak kasir di banyak PC, gunakan
                installer "full" berbasis Postgres.


Backup
------

Tiga cara - semuanya menghasilkan folder
backup_<timestamp>\ di dalam folder backups\ :

a) Dari UI web:    login sebagai OWNER -> Settings -> Backups
                   -> Create. Entri baru muncul di paling atas.

b) Dari command line:  buka Command Prompt di folder ini
                       (Shift + klik kanan folder, lalu
                       "Open command window here") lalu ketik:
                          apotech-portable.exe --backup

c) Jadwalkan tiap malam:  Task Scheduler Windows -> Create
                          Basic Task -> arahkan ke
                          apotech-portable.exe dengan argument
                          --backup .

Tiap backup adalah snapshot SQLite yang bisa Anda buka di
"DB Browser for SQLite" (gratis) untuk dilihat isinya.


Restore
-------

1. Tutup EXE yang sedang jalan.
2. Salin   backups\backup_<TIMESTAMP>\database.db
   ke     data\apotech.db   (timpa file yang ada).
3. Jalankan kembali apotech-portable.exe.

Selesai. Tidak butuh tool tambahan.


Memindahkan instalasi ke PC lain (atau ke USB)
----------------------------------------------

Cukup salin seluruh folder (apotech-portable.exe, config.yaml,
data\, backups\) ke lokasi baru. Klik dua kali EXE di sana -
semua berjalan sama. Kredensial owner, JWT secret, dan data
Anda semuanya ikut di folder ini.

Tips: untuk pemakaian intensif, hindari menjalankan langsung
dari USB. SQLite cocok di SSD/HDD; USB murah sering lambat.
Salin ke disk lokal untuk operasi harian, sinkronkan kembali
ke USB sebagai backup luar.


FAQ
---

T: Apa butuh internet?
J: Tidak. Aplikasi berjalan sepenuhnya di PC ini.

T: Bisakah orang lain di jaringan kantor mengakses?
J: Secara default tidak. EXE diikat ke 127.0.0.1 (PC ini saja).
   Itu menjaga keamanan untuk pemakaian portable. Jika butuh
   akses LAN, pakai installer "Apotech" full.

T: Bagaimana jika port 8080 dipakai?
J: Buka config.yaml di Notepad, ubah server.port ke nomor
   bebas (misal 8090), simpan, jalankan ulang EXE.

T: Saya lupa password owner.
J: Hapus config.yaml DAN folder data\, lalu jalankan
   first-run.bat lagi. PERINGATAN: ini menghapus data Anda.
   Jika ada backup, restore dulu sebelum login. Jika belum
   ada backup, hubungi developer untuk reset password.

T: Di mana log aplikasi?
J: Tercetak ke jendela konsol tempat EXE berjalan.
   Simpan dengan redirect:
     apotech-portable.exe > apotech.log 2>&1


Pemecahan masalah
-----------------

* EXE terbuka lalu langsung tertutup:
  buka Command Prompt di folder ini lalu ketik:
     apotech-portable.exe
  agar pesan error terlihat sebelum jendela hilang.

* "config.yaml not found":
  Anda melewati langkah 1. Klik dua kali first-run.bat.

* "address already in use":
  program lain (atau salinan Apotech Portable lain) memakai
  port 8080. Ubah server.port di config.yaml atau tutup
  program tersebut.

* Browser tidak terbuka otomatis:
  abaikan saja - buka http://127.0.0.1:8080 manual di browser.


Butuh lebih detail?
-------------------

Lihat DEPLOYMENT.md di repository source, atau README
developer di packaging/windows/README-portable.md .

================================================================
