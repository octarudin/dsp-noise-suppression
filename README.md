# DSP Noise Suppression

CLI Go untuk merekam mikrofon mono 48 kHz dan menghasilkan audio mentah serta tiga versi noise suppression klasik berbasis STFT/FFT, tanpa neural network:

1. OM-LSA dengan estimasi noise IMCRA
2. Log-MMSE dengan decision-directed SNR
3. Wiener filter dengan decision-directed SNR

Sebelum STFT, semua metode memakai high-pass 80 Hz dan notch 50/100 Hz. Audio ditulis sebagai WAV PCM 16-bit mono. CPU dan RAM proses diambil setiap 250 ms selama perekaman dan disimpan sebagai CSV.

## Platform dan persyaratan

- Go 1.22 atau lebih baru
- Windows dengan layanan Windows Audio aktif; atau
- Raspberry Pi OS/Linux dengan paket `alsa-utils`
- BOYA BY-MM1+ terhubung ke input audio komputer atau audio interface

BOYA BY-MM1+ adalah mikrofon analog, sehingga nama perangkat yang terlihat biasanya merupakan nama sound card/audio interface, bukan nama mikrofon. Raspberry Pi 4 tidak memiliki input mikrofon analog; hubungkan BOYA melalui USB audio adapter yang menyediakan microphone input.

Pada Windows, program menggunakan Windows Multimedia API secara langsung. Pada Linux/Raspberry Pi, program menggunakan ALSA melalui `arecord`. Keduanya tidak memerlukan CGo.

## Penggunaan

```powershell
go run . --list-devices
go run . --device 0 --duration 10s --output recordings
```

Tanpa `--device`, program memakai input default sistem:

```powershell
go run .
```

### Raspberry Pi 4

Pasang ALSA utilities, lalu periksa perangkat input:

```bash
sudo apt update
sudo apt install -y alsa-utils
go run . --list-devices
go run . --device 0 --duration 10s --output recordings
```

Program memilih perangkat `plughw` agar ALSA dapat menyesuaikan format perangkat ke PCM 16-bit, mono, 48 kHz. Untuk membangun binary Raspberry Pi 4 64-bit dari Windows:

```powershell
$env:GOOS = "linux"
$env:GOARCH = "arm64"
$env:CGO_ENABLED = "0"
go build -o dsp-noise-suppression .
```

Untuk Raspberry Pi OS 32-bit, gunakan `GOARCH=arm` dan `GOARM=7`. Salin binary ke Raspberry Pi, kemudian jalankan:

```bash
chmod +x dsp-noise-suppression
./dsp-noise-suppression --list-devices
./dsp-noise-suppression --device 0 --output recordings
```

Contoh output untuk satu sesi:

```text
recordings/
└── 2026-10-02-15-04-05/
    ├── raw-2026-10-02-15-04-05.wav
    ├── filtered-omlsa-imcra-2026-10-02-15-04-05.wav
    ├── filtered-logmmse-2026-10-02-15-04-05.wav
    ├── filtered-wiener-2026-10-02-15-04-05.wav
    └── metrics-2026-10-02-15-04-05.csv
```

Setiap eksekusi membuat satu subfolder timestamp baru agar hasil beberapa sesi tidak bercampur. Seluruh file di dalamnya memakai timestamp yang sama, dengan komponen tanggal dan waktu dipisahkan tanda strip. Nilai CPU dinormalisasi terhadap seluruh logical CPU. RAM adalah working set proses; pada Linux, metrik turut menghitung proses anak `arecord` selama perekaman.
