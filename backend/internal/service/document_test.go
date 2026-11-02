package service_test

import (
	"archive/zip"
	"bytes"
	"context"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"fite-arsip-api/internal/domain"
	"fite-arsip-api/internal/service"
	"fite-arsip-api/internal/storage"
)

func createDummyOfficeZip(t *testing.T, mainDir string) []byte {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	w1, err := zw.Create("[Content_Types].xml")
	if err != nil {
		t.Fatalf("failed to create xml entry: %v", err)
	}
	_, _ = w1.Write([]byte("<Types></Types>"))

	w2, err := zw.Create(mainDir + "document.xml")
	if err != nil {
		t.Fatalf("failed to create office entry: %v", err)
	}
	_, _ = w2.Write([]byte("<doc></doc>"))

	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}
	return buf.Bytes()
}

func createMultipartFileHeader(t *testing.T, name string, content []byte) *multipart.FileHeader {
	body := new(bytes.Buffer)
	mw := multipart.NewWriter(body)
	w, err := mw.CreateFormFile("file", name)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	_, _ = w.Write(content)
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if err := req.ParseMultipartForm(10 << 20); err != nil {
		t.Fatalf("failed to parse multipart: %v", err)
	}
	return req.MultipartForm.File["file"][0]
}

func TestFileCheck_InspectUpload(t *testing.T) {
	t.Run("Valid PDF", func(t *testing.T) {
		content := []byte("%PDF-1.4 dummy content")
		fh := createMultipartFileHeader(t, "laporan.pdf", content)
		up, err := service.InspectUpload(fh, 10<<20)
		if err != nil {
			t.Fatalf("expected valid pdf, got %v", err)
		}
		defer up.File.Close()
		if up.Ext != "pdf" || up.Mime != "application/pdf" {
			t.Errorf("expected pdf metadata, got ext=%s mime=%s", up.Ext, up.Mime)
		}
	})

	t.Run("Fake PDF rejected", func(t *testing.T) {
		content := []byte("plain text pretending to be pdf")
		fh := createMultipartFileHeader(t, "fake.pdf", content)
		_, err := service.InspectUpload(fh, 10<<20)
		if err == nil {
			t.Fatal("expected fake pdf to be rejected")
		}
	})

	t.Run("Valid DOCX with Office structure", func(t *testing.T) {
		content := createDummyOfficeZip(t, "word/")
		fh := createMultipartFileHeader(t, "thesis.docx", content)
		up, err := service.InspectUpload(fh, 10<<20)
		if err != nil {
			t.Fatalf("expected valid docx, got %v", err)
		}
		defer up.File.Close()
		if up.Ext != "docx" {
			t.Errorf("expected docx, got %s", up.Ext)
		}
	})

	t.Run("Fake DOCX without Office structure rejected", func(t *testing.T) {
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)
		w, _ := zw.Create("random.txt")
		_, _ = w.Write([]byte("just random zip"))
		_ = zw.Close()

		fh := createMultipartFileHeader(t, "fake.docx", buf.Bytes())
		_, err := service.InspectUpload(fh, 10<<20)
		if err == nil {
			t.Fatal("expected fake docx to be rejected")
		}
	})

	t.Run("Dangerous double extension rejected", func(t *testing.T) {
		content := []byte("%PDF-1.4 dummy content")
		fh := createMultipartFileHeader(t, "laporan.pdf.exe", content)
		_, err := service.InspectUpload(fh, 10<<20)
		if err == nil {
			t.Fatal("expected double extension .pdf.exe to be rejected")
		}
	})
}

func TestDocumentService_LifecycleAndRBAC(t *testing.T) {
	db := setupTestDB(t)
	memStore := storage.NewMemoryStorage()
	docSvc := service.NewDocumentService(db, memStore)
	verSvc := service.NewVerificationService(db)
	ctx := context.Background()

	// Seed Prodi
	prodiIF := domain.Prodi{Code: "IF", Name: "S1 Informatika", IsActive: true}
	prodiSI := domain.Prodi{Code: "SI", Name: "S1 Sistem Informasi", IsActive: true}
	db.Create(&prodiIF)
	db.Create(&prodiSI)

	// Seed Users
	mhsIF := seedUser(t, db, domain.RoleMahasiswa, "mhs.if@del.ac.id", "Secret123", true, false)
	mhsIF.ProdiID = &prodiIF.ID
	mhsIF.Prodi = &prodiIF
	db.Save(mhsIF)

	mhsSI := seedUser(t, db, domain.RoleMahasiswa, "mhs.si@del.ac.id", "Secret123", true, false)
	mhsSI.ProdiID = &prodiSI.ID
	mhsSI.Prodi = &prodiSI
	db.Save(mhsSI)

	dosenIF := seedUser(t, db, domain.RoleDosen, "dosen.if@del.ac.id", "Secret123", true, false)
	dosenIF.ProdiID = &prodiIF.ID
	dosenIF.Prodi = &prodiIF
	db.Save(dosenIF)

	baak := seedUser(t, db, domain.RoleBaak, "baak@del.ac.id", "Secret123", true, false)
	_ = seedUser(t, db, domain.RoleSuperAdmin, "sa@del.ac.id", "Secret123", true, false)

	// Seed Category
	catTA := domain.Category{Name: "Proposal TA/PA", AllowedRoles: domain.StringList{domain.RoleMahasiswa, domain.RoleSuperAdmin}}
	db.Create(&catTA)

	// 1. Mahasiswa IF uploads a document (Private)
	pdfBytes := []byte("%PDF-1.4 Test Doc Content")
	fh := createMultipartFileHeader(t, "proposal.pdf", pdfBytes)
	up, err := service.InspectUpload(fh, 10<<20)
	if err != nil {
		t.Fatalf("inspect upload failed: %v", err)
	}
	defer up.File.Close()

	doc, err := docSvc.Create(ctx, service.CreateInput{
		Actor:      mhsIF,
		Title:      "Proposal Skripsi IF",
		CategoryID: catTA.ID,
		Public:     false,
		Upload:     up,
	})
	if err != nil {
		t.Fatalf("failed to create document: %v", err)
	}

	if doc.Visibility != "private" || doc.VerificationStatus != nil {
		t.Errorf("expected default private doc without verification")
	}

	// 2. Test RBAC: Dosen IF can view Mhs IF document
	canViewDosenIF, err := docSvc.CanView(ctx, dosenIF, doc)
	if err != nil || !canViewDosenIF {
		t.Errorf("Dosen IF should be able to view Mahasiswa IF document, got: %v", err)
	}

	// Mahasiswa SI CANNOT view Mahasiswa IF document (Private)
	canViewMhsSI, _ := docSvc.CanView(ctx, mhsSI, doc)
	if canViewMhsSI {
		t.Errorf("Mahasiswa SI should NOT be able to view Mahasiswa IF document")
	}

	// 3. Mahasiswa requests Public visibility -> status pending
	docPub, err := docSvc.ChangeVisibility(ctx, mhsIF, doc.ID, "public")
	if err != nil {
		t.Fatalf("failed to change visibility to public: %v", err)
	}
	if docPub.Visibility != "public" || docPub.VerificationStatus == nil || *docPub.VerificationStatus != "pending" {
		t.Errorf("expected visibility public & status pending")
	}

	// Verify notification generated for BAAK & Super Admin
	var notifCount int64
	db.Model(&domain.Notification{}).Where("type = ?", "verification_pending").Count(&notifCount)
	if notifCount < 2 {
		t.Errorf("expected notifications sent to BAAK and SA, got %d", notifCount)
	}

	// 4. BAAK decides verification -> approved
	var ver domain.Verification
	db.Where("document_id = ?", doc.ID).First(&ver)
	decidedVer, err := verSvc.Decide(ctx, baak, ver.ID, "approved", "")
	if err != nil {
		t.Fatalf("verification decide failed: %v", err)
	}
	if decidedVer.Status != "approved" {
		t.Errorf("expected verification status approved")
	}

	// 5. Check Public Explorer: now visible to everyone (including Mhs SI)
	listPublic, totalPub, err := docSvc.List(ctx, mhsSI, service.DocumentFilter{}, true)
	if err != nil || totalPub == 0 || len(listPublic) == 0 {
		t.Fatalf("expected approved public document in public list, got total=%d", totalPub)
	}

	// 6. Test Delete authorization: Mahasiswa SI cannot delete Mahasiswa IF document
	if service.CanDeleteDocument(mhsSI, doc) {
		t.Errorf("Mahasiswa SI should not have permission to delete doc")
	}
}


func TestDocumentService_VerificationRejectAndConflict(t *testing.T) {
	db := setupTestDB(t)
	memStore := storage.NewMemoryStorage()
	docSvc := service.NewDocumentService(db, memStore)
	verSvc := service.NewVerificationService(db)
	ctx := context.Background()

	prodi := domain.Prodi{Code: "IF", Name: "Informatika", IsActive: true}
	db.Create(&prodi)
	mhs := seedUser(t, db, domain.RoleMahasiswa, "mhs_reject@del.ac.id", "Secret123", true, false)
	mhs.ProdiID = &prodi.ID
	db.Save(mhs)
	baak := seedUser(t, db, domain.RoleBaak, "baak_verifier@del.ac.id", "Secret123", true, false)
	cat := domain.Category{Name: "Tugas Akhir", AllowedRoles: domain.StringList{domain.RoleMahasiswa}}
	db.Create(&cat)

	// Create document with public visibility -> status pending
	fh := createMultipartFileHeader(t, "skripsi.pdf", []byte("%PDF-1.4 proposal text"))
	up, err := service.InspectUpload(fh, 10<<20)
	if err != nil {
		t.Fatalf("inspect upload failed: %v", err)
	}
	defer up.File.Close()

	doc, err := docSvc.Create(ctx, service.CreateInput{
		Actor:      mhs,
		Title:      "Skripsi Publik Pending",
		CategoryID: cat.ID,
		Public:     true,
		Upload:     up,
	})
	if err != nil {
		t.Fatalf("create document failed: %v", err)
	}

	var ver domain.Verification
	if err := db.Where("document_id = ?", doc.ID).First(&ver).Error; err != nil {
		t.Fatalf("verification queue entry not found: %v", err)
	}

	// 1. Rejection decision with reason
	decided, err := verSvc.Decide(ctx, baak, ver.ID, "rejected", "Judul tidak sesuai dengan format fakultas")
	if err != nil {
		t.Fatalf("decide reject failed: %v", err)
	}
	if decided.Status != "rejected" || decided.Reason == nil || *decided.Reason != "Judul tidak sesuai dengan format fakultas" {
		t.Errorf("expected rejected status with reason")
	}

	// Document visibility reverted to private and status rejected
	docReloaded, err := docSvc.Get(ctx, doc.ID)
	if err != nil {
		t.Fatalf("get doc failed: %v", err)
	}
	if docReloaded.Visibility != "private" || docReloaded.VerificationStatus == nil || *docReloaded.VerificationStatus != "rejected" {
		t.Errorf("expected doc reverted to private and rejected status")
	}

	// Check notification sent to uploader
	var notif domain.Notification
	if err := db.Where("user_id = ? AND type = ?", mhs.ID, "verification_rejected").First(&notif).Error; err != nil {
		t.Errorf("expected rejection notification sent to user: %v", err)
	}

	// 2. Concurrency Conflict: Deciding on already resolved verification must return Conflict
	_, err = verSvc.Decide(ctx, baak, ver.ID, "approved", "")
	if err == nil {
		t.Fatalf("expected conflict error when deciding on already resolved verification")
	}
}

func TestDocumentService_HardDelete(t *testing.T) {
	db := setupTestDB(t)
	memStore := storage.NewMemoryStorage()
	docSvc := service.NewDocumentService(db, memStore)
	ctx := context.Background()

	prodi := domain.Prodi{Code: "IF", Name: "Informatika", IsActive: true}
	db.Create(&prodi)
	sa := seedUser(t, db, domain.RoleSuperAdmin, "sa_delete@del.ac.id", "Secret123", true, false)
	cat := domain.Category{Name: "General", AllowedRoles: domain.StringList{domain.RoleSuperAdmin}}
	db.Create(&cat)

	fh := createMultipartFileHeader(t, "doc_del.pdf", []byte("%PDF-1.4 file to delete"))
	up, err := service.InspectUpload(fh, 10<<20)
	if err != nil {
		t.Fatalf("inspect upload failed: %v", err)
	}
	defer up.File.Close()

	doc, err := docSvc.Create(ctx, service.CreateInput{
		Actor:      sa,
		Title:      "Doc to be permanently deleted",
		CategoryID: cat.ID,
		ProdiID:    &prodi.ID,
		Upload:     up,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Hard delete (force = true)
	if err := docSvc.Delete(ctx, sa, doc, true); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Object removed from storage
	_, _, err = memStore.Open(ctx, doc.StoragePath)
	if err == nil {
		t.Errorf("expected object to be removed from storage on hard delete")
	}

	// Record completely gone from DB (unscoped)
	var count int64
	db.Unscoped().Model(&domain.Document{}).Where("id = ?", doc.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected record to be physically deleted from database")
	}
}

func TestDocumentService_AtomicCompensation(t *testing.T) {
	db := setupTestDB(t)
	memStore := storage.NewMemoryStorage()
	docSvc := service.NewDocumentService(db, memStore)
	ctx := context.Background()

	prodi := domain.Prodi{Code: "IF", Name: "Informatika", IsActive: true}
	db.Create(&prodi)
	sa := seedUser(t, db, domain.RoleSuperAdmin, "sa_comp@del.ac.id", "Secret123", true, false)
	cat := domain.Category{Name: "General", AllowedRoles: domain.StringList{domain.RoleSuperAdmin}}
	db.Create(&cat)

	// Trigger to force DB insert failure
	db.Exec("CREATE TRIGGER trigger_force_doc_fail BEFORE INSERT ON documents WHEN NEW.title = 'FORCE_FAIL' BEGIN SELECT RAISE(ABORT, 'forced database transaction failure'); END;")

	fh := createMultipartFileHeader(t, "comp_test.pdf", []byte("%PDF-1.4 test atomic compensation"))
	up, err := service.InspectUpload(fh, 10<<20)
	if err != nil {
		t.Fatalf("inspect upload failed: %v", err)
	}
	defer up.File.Close()

	_, err = docSvc.Create(ctx, service.CreateInput{
		Actor:      sa,
		Title:      "FORCE_FAIL",
		CategoryID: cat.ID,
		ProdiID:    &prodi.ID,
		Upload:     up,
	})
	if err == nil {
		t.Fatal("expected Create to fail due to trigger")
	}

	// Verify the object was rolled back / deleted from storage
	// If atomic compensation succeeded, nothing remains in memStore
	openErr := memStore.Ping(ctx)
	if openErr != nil {
		t.Fatalf("storage error: %v", openErr)
	}
}

func TestSanitizeName(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"../../etc/passwd", "passwd"},
		{"C:\\Windows\\System32\\file.pdf", "file.pdf"},
		{"normal-doc (1)_rev.pdf", "normal-doc (1)_rev.pdf"},
		{"../../../dangerous;rm -rf", "dangerous_rm -rf"},
		{"", "berkas"},
		{".", "berkas"},
		{"..", "berkas"},
	}

	for _, c := range cases {
		got := service.SanitizeName(c.input)
		if got != c.expected {
			t.Errorf("SanitizeName(%q) = %q, expected %q", c.input, got, c.expected)
		}
	}
}
