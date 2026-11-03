INSERT INTO users (name, email, password, role_id, prodi_id, nim, nip, is_active, must_change_password) VALUES
('Super Admin', 'admin@del.ac.id', '$2a$12$SULIb9cQnCV2ytSEIbXvCeHQnwr5LwWnTS32TmSkUPUtcMvgORbMK', (SELECT id FROM roles WHERE `key` = 'super_admin'), NULL, NULL, NULL, 1, 0),
('Dekan FITE', 'dekan@del.ac.id', '$2a$12$SULIb9cQnCV2ytSEIbXvCeHQnwr5LwWnTS32TmSkUPUtcMvgORbMK', (SELECT id FROM roles WHERE `key` = 'dekan'), NULL, NULL, NULL, 1, 0),
('Staf BAAK', 'baak@del.ac.id', '$2a$12$SULIb9cQnCV2ytSEIbXvCeHQnwr5LwWnTS32TmSkUPUtcMvgORbMK', (SELECT id FROM roles WHERE `key` = 'baak'), NULL, NULL, NULL, 1, 0),
('Kaprodi Informatika', 'kaprodi.if@del.ac.id', '$2a$12$SULIb9cQnCV2ytSEIbXvCeHQnwr5LwWnTS32TmSkUPUtcMvgORbMK', (SELECT id FROM roles WHERE `key` = 'kaprodi'), (SELECT id FROM prodi WHERE code = 'IF'), NULL, '198001012005011001', 1, 0),
('Dosen Informatika', 'dosen.if@del.ac.id', '$2a$12$SULIb9cQnCV2ytSEIbXvCeHQnwr5LwWnTS32TmSkUPUtcMvgORbMK', (SELECT id FROM roles WHERE `key` = 'dosen'), (SELECT id FROM prodi WHERE code = 'IF'), NULL, '198505052010011002', 1, 0),
('Budi Mahasiswa', 'budi@del.ac.id', '$2a$12$SULIb9cQnCV2ytSEIbXvCeHQnwr5LwWnTS32TmSkUPUtcMvgORbMK', (SELECT id FROM roles WHERE `key` = 'mahasiswa'), (SELECT id FROM prodi WHERE code = 'IF'), '11S20001', NULL, 1, 0),
('Mahasiswa Sistem Informasi', 'iss24034@del.ac.id', '$2a$12$SULIb9cQnCV2ytSEIbXvCeHQnwr5LwWnTS32TmSkUPUtcMvgORbMK', (SELECT id FROM roles WHERE `key` = 'mahasiswa'), (SELECT id FROM prodi WHERE code = 'SI'), 'iss24034', NULL, 1, 0)
ON DUPLICATE KEY UPDATE name=VALUES(name), password=VALUES(password), role_id=VALUES(role_id), prodi_id=VALUES(prodi_id);
