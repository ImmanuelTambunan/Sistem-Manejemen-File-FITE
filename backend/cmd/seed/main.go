package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"fite-arsip-api/internal/app"
	"fite-arsip-api/internal/config"
	"fite-arsip-api/internal/domain"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm/clause"
)

func main() {
	mustChangePwFlag := flag.Bool("must-change-pw", false, "set must_change_password=true for initial testing of S-02")
	flag.Parse()

	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("gagal memuat konfigurasi: %v", err)
	}

	db, err := app.OpenDB(cfg)
	if err != nil {
		log.Fatalf("gagal menghubungkan ke database: %v", err)
	}

	const passwordPlain = "Password123!"
	pwHash, err := bcrypt.GenerateFromPassword([]byte(passwordPlain), 12)
	if err != nil {
		log.Fatalf("gagal membuat hash password: %v", err)
	}
	hashStr := string(pwHash)

	// Fetch roles
	var roles []domain.Role
	if err := db.Find(&roles).Error; err != nil || len(roles) == 0 {
		log.Fatalf("roles belum di-seed, jalankan migrasi 000010 terlebih dahulu: %v", err)
	}
	roleMap := make(map[string]uint)
	for _, r := range roles {
		roleMap[r.Key] = r.ID
	}

	// Fetch prodis
	var prodis []domain.Prodi
	if err := db.Find(&prodis).Error; err != nil {
		log.Fatalf("gagal membaca prodi: %v", err)
	}
	prodiMap := make(map[string]uint)
	for _, p := range prodis {
		prodiMap[p.Code] = p.ID
	}

	prodiIF := prodiMap["IF"]
	prodiSI := prodiMap["SI"]
	prodiTE := prodiMap["TE"]
	_ = prodiTE

	strPtr := func(s string) *string { return &s }
	uintPtr := func(u uint) *uint { return &u }

	// 10 Akun Uji Coba Resmi SRS FITE (Bab 3.0 & 2.4):
	// SA, DKN, BAK, KPR-IF, KPR-SI, DSN-IF, DSN-SI, MHS-IF-1, MHS-IF-2, MHS-SI-1
	users := []domain.User{
		{
			Name:               "Super Admin",
			Email:              "admin@del.ac.id",
			PasswordHash:       hashStr,
			RoleID:             roleMap[domain.RoleSuperAdmin],
			IsActive:           true,
			MustChangePassword: *mustChangePwFlag,
		},
		{
			Name:               "Dekan FITE",
			Email:              "dekan@del.ac.id",
			PasswordHash:       hashStr,
			RoleID:             roleMap[domain.RoleDekan],
			IsActive:           true,
			MustChangePassword: *mustChangePwFlag,
		},
		{
			Name:               "Staf BAAK",
			Email:              "baak@del.ac.id",
			PasswordHash:       hashStr,
			RoleID:             roleMap[domain.RoleBaak],
			IsActive:           true,
			MustChangePassword: *mustChangePwFlag,
		},
		{
			Name:               "Kaprodi Informatika",
			Email:              "kaprodi.if@del.ac.id",
			PasswordHash:       hashStr,
			RoleID:             roleMap[domain.RoleKaprodi],
			ProdiID:            uintPtr(prodiIF),
			NIP:                strPtr("198001012005011001"),
			IsActive:           true,
			MustChangePassword: *mustChangePwFlag,
		},
		{
			Name:               "Kaprodi Sistem Informasi",
			Email:              "kaprodi.si@del.ac.id",
			PasswordHash:       hashStr,
			RoleID:             roleMap[domain.RoleKaprodi],
			ProdiID:            uintPtr(prodiSI),
			NIP:                strPtr("198202022008011003"),
			IsActive:           true,
			MustChangePassword: *mustChangePwFlag,
		},
		{
			Name:               "Dosen Informatika",
			Email:              "dosen.if@del.ac.id",
			PasswordHash:       hashStr,
			RoleID:             roleMap[domain.RoleDosen],
			ProdiID:            uintPtr(prodiIF),
			NIP:                strPtr("198505052010011002"),
			IsActive:           true,
			MustChangePassword: *mustChangePwFlag,
		},
		{
			Name:               "Dosen Sistem Informasi",
			Email:              "dosen.si@del.ac.id",
			PasswordHash:       hashStr,
			RoleID:             roleMap[domain.RoleDosen],
			ProdiID:            uintPtr(prodiSI),
			NIP:                strPtr("198707072012011004"),
			IsActive:           true,
			MustChangePassword: *mustChangePwFlag,
		},
		{
			Name:               "Budi Mahasiswa (IF-1)",
			Email:              "budi@del.ac.id",
			PasswordHash:       hashStr,
			RoleID:             roleMap[domain.RoleMahasiswa],
			ProdiID:            uintPtr(prodiIF),
			NIM:                strPtr("11S20001"),
			IsActive:           true,
			MustChangePassword: *mustChangePwFlag,
		},
		{
			Name:               "Mahasiswa Informatika (IF-2)",
			Email:              "mhs.if2@del.ac.id",
			PasswordHash:       hashStr,
			RoleID:             roleMap[domain.RoleMahasiswa],
			ProdiID:            uintPtr(prodiIF),
			NIM:                strPtr("11S20002"),
			IsActive:           true,
			MustChangePassword: *mustChangePwFlag,
		},
		{
			Name:               "Mahasiswa Sistem Informasi (SI-1)",
			Email:              "iss24034@del.ac.id",
			PasswordHash:       hashStr,
			RoleID:             roleMap[domain.RoleMahasiswa],
			ProdiID:            uintPtr(prodiSI),
			NIM:                strPtr("iss24034"),
			IsActive:           true,
			MustChangePassword: *mustChangePwFlag,
		},
		{
			Name:               "Kaprodi Teknik Elektro",
			Email:              "kaprodi.te@del.ac.id",
			PasswordHash:       hashStr,
			RoleID:             roleMap[domain.RoleKaprodi],
			ProdiID:            uintPtr(prodiTE),
			NIP:                strPtr("198303032009011005"),
			IsActive:           true,
			MustChangePassword: *mustChangePwFlag,
		},
		{
			Name:               "Dosen Teknik Elektro",
			Email:              "dosen.te@del.ac.id",
			PasswordHash:       hashStr,
			RoleID:             roleMap[domain.RoleDosen],
			ProdiID:            uintPtr(prodiTE),
			NIP:                strPtr("198909092014011006"),
			IsActive:           true,
			MustChangePassword: *mustChangePwFlag,
		},
		{
			Name:               "Mahasiswa Teknik Elektro (TE-1)",
			Email:              "mhs.te@del.ac.id",
			PasswordHash:       hashStr,
			RoleID:             roleMap[domain.RoleMahasiswa],
			ProdiID:            uintPtr(prodiTE),
			NIM:                strPtr("14S20001"),
			IsActive:           true,
			MustChangePassword: *mustChangePwFlag,
		},
	}

	fmt.Println("=================================================================")
	fmt.Println("             SIFITE TEST ACCOUNTS SEEDER (10 AKUN SRS)           ")
	fmt.Println("=================================================================")

	for _, u := range users {
		err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "email"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "password", "role_id", "prodi_id", "nim", "nip", "is_active", "must_change_password"}),
		}).Create(&u).Error

		if err != nil {
			fmt.Printf("[-] Gagal seed user %s: %v\n", u.Email, err)
		} else {
			fmt.Printf("[+] Berhasil seed user: %-25s | Peran: %-12s | Pass: %s\n", u.Email, u.Role.Key, passwordPlain)
		}
	}

	fmt.Println("=================================================================")
	fmt.Println("Semua 10 akun pengujian SRS siap digunakan untuk login!")
	fmt.Printf("Bcrypt Work Factor 12 Hash: %s\n", hashStr)
	os.Exit(0)
}
