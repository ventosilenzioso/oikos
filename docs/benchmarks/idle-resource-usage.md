# Idle Resource Usage

## Scope

Pengukuran ini memvalidasi target idle Fase 1 untuk daemon Oikos tanpa server
aktif. Pengukuran bukan perbandingan dengan Wings dan bukan load test 10/50/100
server; kedua hal tersebut masuk validasi production hardening Fase 6.

## Metodologi

- Host: Linux amd64, kernel dengan cgroup v2.
- Binary: `oikos` dibangun dari source workspace ini.
- Panel: `cmd/mock-panel`, berjalan di localhost.
- Runtime: Docker daemon tersedia, tetapi tidak ada container server aktif.
- Sampel: 12 sampel, satu setiap 5 detik, setelah warm-up 2 detik.
- RSS: `VmRSS` dari `/proc/<pid>/status`.
- CPU: delta `utime + stime` dari `/proc/<pid>/stat`, dengan asumsi kernel
  100 jiffies/detik.

## Hasil

Tanggal pengukuran: 2026-09-13

| Kondisi | RSS min | RSS max | RSS rata-rata | CPU max | CPU rata-rata |
|---|---:|---:|---:|---:|---:|
| 0 server, daemon idle | 20.56 MB | 20.66 MB | 20.64 MB | 0.20% | 0.02% |

Nilai RSS dikonversi dari hasil `VmRSS` berikut:

```text
RSS_KB min=21052 max=21160 avg=21138
CPU_PCT max=0.20 avg=0.02
```

## Kesimpulan

Target awal Fase 1 terpenuhi pada skenario idle:

- RAM: `20.64 MB < 30 MB`
- CPU rata-rata: `0.02% < 1%`
- CPU maksimum sampel: `0.20% < 1%`

Hasil ini adalah satu baseline pada satu host, bukan klaim universal. Pengujian
dengan server aktif, jumlah node lebih besar, perbandingan Wings, dan profiling
lanjutan harus dilakukan pada Fase 6.
