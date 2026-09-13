# Disaster Recovery

1. Hentikan daemon, jangan menghapus container Docker yang masih diperlukan.
2. Simpan salinan `config.yaml`, SQLite, cert/key, plugin manifest, dan slot
   binary terakhir.
3. Pulihkan binary slot yang tervalidasi melalui `current`/`state.json`.
4. Jalankan `oikos doctor` sebelum menyalakan daemon.
5. Jika SQLite korup, pulihkan backup database terakhir dan jalankan migration.
6. Verifikasi node pairing dan cert sebelum koneksi Panel dibuka.
7. Verifikasi semua server/container melalui Docker sebelum mengirim command.

Restore file server memakai backup checksum-valid dan server harus stopped.
Dokumen ini tidak menjanjikan pemulihan tanpa backup atau live migration.
