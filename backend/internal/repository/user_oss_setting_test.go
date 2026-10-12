package repository

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"yingce/backend/internal/model"
)

func TestUserOSSSettingVersionsRemainOrderedWhenClockDoesNotAdvance(t *testing.T) {
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{NowFunc: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	if err := db.AutoMigrate(&model.UserOSSSetting{}); err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	older := model.UserOSSSetting{ID: "z-older", UserID: "user-1"}
	newer := model.UserOSSSetting{ID: "a-newer", UserID: "user-1"}
	for _, setting := range []*model.UserOSSSetting{&older, &newer} {
		if err := repo.CreateUserOSSSetting(setting); err != nil {
			t.Fatal(err)
		}
	}
	latest, err := repo.LatestUserOSSSetting("user-1")
	if err != nil {
		t.Fatal(err)
	}
	if latest.ID != newer.ID {
		t.Fatalf("latest version = %q, want %q", latest.ID, newer.ID)
	}
	versions, err := repo.UserOSSSettingsForUser("user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 || versions[0].ID != newer.ID || versions[1].ID != older.ID {
		t.Fatalf("unexpected version order: %#v", versions)
	}
}
