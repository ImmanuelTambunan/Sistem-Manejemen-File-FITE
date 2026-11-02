INSERT INTO roles (`key`, name) VALUES
('super_admin', 'Super Admin'),
('dekan', 'Dekan'),
('baak', 'BAAK / Staf'),
('kaprodi', 'Kaprodi'),
('dosen', 'Dosen / TA'),
('mahasiswa', 'Mahasiswa');

INSERT INTO prodi (code, name) VALUES
('IF', 'S1 Informatika'),
('SI', 'S1 Sistem Informasi'),
('TE', 'S1 Teknik Elektro');

INSERT INTO categories (name, allowed_roles, is_operational) VALUES
('SK Keputusan Dekan', JSON_ARRAY('dekan','super_admin'), 0),
('Dokumen Akreditasi Fakultas', JSON_ARRAY('dekan','super_admin'), 0),
('Arsip Khusus', JSON_ARRAY('dekan','super_admin'), 0),
('Legalisir', JSON_ARRAY('baak','super_admin'), 1),
('Transkrip Nilai Akademik', JSON_ARRAY('baak','super_admin'), 1),
('Dokumen BAAK', JSON_ARRAY('baak','super_admin'), 1),
('Berkas Mahasiswa/Dosen', JSON_ARRAY('baak','super_admin'), 1),
('SK Keputusan Prodi', JSON_ARRAY('kaprodi','super_admin'), 0),
('Dokumen Akreditasi Prodi', JSON_ARRAY('kaprodi','super_admin'), 0),
('Kurikulum', JSON_ARRAY('kaprodi','super_admin'), 0),
('Rencana Pembelajaran Semester (RPS)', JSON_ARRAY('dosen','super_admin'), 0),
('SK Mengajar', JSON_ARRAY('dosen','super_admin'), 0),
('Berita Acara Perkuliahan (BAP)', JSON_ARRAY('dosen','super_admin'), 0),
('Proposal TA/PA', JSON_ARRAY('mahasiswa','super_admin'), 0),
('Berkas Yudisium', JSON_ARRAY('mahasiswa','super_admin'), 0),
('Surat Keterangan', JSON_ARRAY('mahasiswa','super_admin'), 0);
