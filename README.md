# DSP Noise Suppression

CLI Go untuk merekam mikrofon mono 48 kHz dan menghasilkan audio mentah serta tiga versi noise suppression klasik berbasis STFT/FFT, tanpa neural network:

1. OM-LSA dengan estimasi noise IMCRA
2. Log-MMSE dengan decision-directed SNR
3. Wiener filter dengan decision-directed SNR

Sebelum STFT, semua metode memakai high-pass 80 Hz dan notch 50/100 Hz. Audio ditulis sebagai WAV PCM 16-bit mono. CPU dan RAM proses diambil setiap 250 ms selama perekaman dan disimpan sebagai CSV.

## Persyaratan

- Go 1.22 atau lebih baru
- Windows dengan layanan Windows Audio aktif
- BOYA BY-MM1+ terhubung ke input audio komputer atau audio interface

BOYA BY-MM1+ adalah mikrofon analog, sehingga nama perangkat yang terlihat biasanya merupakan nama sound card/audio interface, bukan nama mikrofon.

Program menggunakan Windows Multimedia API secara langsung dan tidak memerlukan CGo maupun library audio eksternal.

## Penggunaan

```powershell
go run . --list-devices
go run . --device 0 --duration 10s --output recordings
```

Tanpa `--device`, program memakai input default sistem:

```powershell
go run .
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

Setiap eksekusi membuat satu subfolder timestamp baru agar hasil beberapa sesi tidak bercampur. Seluruh file di dalamnya memakai timestamp yang sama, dengan komponen tanggal dan waktu dipisahkan tanda strip. Nilai CPU dinormalisasi terhadap seluruh logical CPU. RAM pada Windows adalah working set proses.
