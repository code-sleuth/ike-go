package repository

import (
	"testing"
	"time"

	"github.com/code-sleuth/ike-go/internal/manager/models"
	"github.com/code-sleuth/ike-go/internal/manager/testutil"
	"github.com/code-sleuth/ike-go/pkg/db"
)

func TestSourceRepository_Create_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	testDB := testutil.SetupTestDB(t)
	defer testutil.CleanupTestDB(t, testDB)

	// Create DB wrapper
	dbWrapper := &db.DB{DB: testDB}
	repo := NewSourceRepository(dbWrapper)

	tests := []struct {
		name        string
		source      *models.Source
		expectError bool
		description string
	}{
		{
			name: "create GitHub source",
			source: &models.Source{
				ID:           "github-source-123",
				AuthorEmail:  stringPtrInteg("developer@example.com"),
				RawURL:       stringPtrInteg("https://github.com/owner/repo"),
				Scheme:       stringPtrInteg("https"),
				Host:         stringPtrInteg("github.com"),
				Path:         stringPtrInteg("/owner/repo"),
				ActiveDomain: 1,
				Format:       stringPtrInteg("json"),
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
			expectError: false,
			description: "should create GitHub source successfully",
		},
		{
			name: "create WordPress source",
			source: &models.Source{
				ID:           "wp-source-456",
				AuthorEmail:  stringPtrInteg("admin@blog.com"),
				RawURL:       stringPtrInteg("https://blog.com/wp-json/wp/v2/posts"),
				Scheme:       stringPtrInteg("https"),
				Host:         stringPtrInteg("blog.com"),
				Path:         stringPtrInteg("/wp-json/wp/v2/posts"),
				Query:        stringPtrInteg("per_page=100"),
				ActiveDomain: 1,
				Format:       stringPtrInteg("json"),
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
			expectError: false,
			description: "should create WordPress source successfully",
		},
		{
			name: "create minimal source",
			source: &models.Source{
				ID:           "minimal-source-789",
				ActiveDomain: 0,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
			expectError: false,
			description: "should create minimal source successfully",
		},
		{
			name: "create source with invalid format",
			source: &models.Source{
				ID:           "invalid-format-source",
				Format:       stringPtrInteg("invalid"), // Not in allowed values
				ActiveDomain: 1,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
			expectError: true,
			description: "should fail with invalid format constraint",
		},
		{
			name: "create source with invalid active domain",
			source: &models.Source{
				ID:           "invalid-domain-source",
				ActiveDomain: 2, // Not 0 or 1
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
			expectError: true,
			description: "should fail with invalid active domain constraint",
		},
		{
			name: "create duplicate source",
			source: &models.Source{
				ID:           "github-source-123", // Same as first test
				ActiveDomain: 1,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
			expectError: true,
			description: "should fail with duplicate ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(tt.source)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
			}

			// If creation was successful, verify the source exists
			if !tt.expectError && err == nil {
				retrievedSource, getErr := repo.GetByID(tt.source.ID)
				if getErr != nil {
					t.Errorf("Failed to retrieve created source: %v", getErr)
				} else {
					if retrievedSource.ID != tt.source.ID {
						t.Errorf("Expected ID %s, got %s", tt.source.ID, retrievedSource.ID)
					}
					if retrievedSource.ActiveDomain != tt.source.ActiveDomain {
						t.Errorf("Expected active domain %d, got %d", tt.source.ActiveDomain, retrievedSource.ActiveDomain)
					}
				}
			}
		})
	}
}

func TestSourceRepository_GetByID_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	testDB := testutil.SetupTestDB(t)
	defer testutil.CleanupTestDB(t, testDB)

	// Create DB wrapper
	dbWrapper := &db.DB{DB: testDB}
	repo := NewSourceRepository(dbWrapper)

	// Create a test source first
	testSource := &models.Source{
		ID:           "test-get-source",
		AuthorEmail:  stringPtrInteg("test@example.com"),
		RawURL:       stringPtrInteg("https://example.com/api"),
		Scheme:       stringPtrInteg("https"),
		Host:         stringPtrInteg("example.com"),
		Path:         stringPtrInteg("/api"),
		Query:        stringPtrInteg("format=json"),
		ActiveDomain: 1,
		Format:       stringPtrInteg("json"),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := repo.Create(testSource)
	if err != nil {
		t.Fatalf("Failed to create test source: %v", err)
	}

	tests := []struct {
		name        string
		sourceID    string
		expectError bool
		expectedErr error
		description string
	}{
		{
			name:        "get existing source",
			sourceID:    "test-get-source",
			expectError: false,
			description: "should retrieve existing source",
		},
		{
			name:        "get nonexistent source",
			sourceID:    "nonexistent-source",
			expectError: true,
			expectedErr: errSourceNotFound,
			description: "should return error for nonexistent source",
		},
		{
			name:        "get with empty ID",
			sourceID:    "",
			expectError: true,
			description: "should handle empty source ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := repo.GetByID(tt.sourceID)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
			}
			if tt.expectedErr != nil && err != tt.expectedErr {
				t.Errorf("Expected error %v, got %v", tt.expectedErr, err)
			}

			// If successful, verify the returned source
			if !tt.expectError && err == nil {
				if source == nil {
					t.Error("Expected non-nil source")
				} else {
					if source.ID != tt.sourceID {
						t.Errorf("Expected ID %s, got %s", tt.sourceID, source.ID)
					}
					if source.AuthorEmail == nil || *source.AuthorEmail != "test@example.com" {
						t.Error("Author email not retrieved correctly")
					}
					if source.RawURL == nil || *source.RawURL != "https://example.com/api" {
						t.Error("Raw URL not retrieved correctly")
					}
				}
			}
		})
	}
}

func TestSourceRepository_List_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	testDB := testutil.SetupTestDB(t)
	defer testutil.CleanupTestDB(t, testDB)

	// Create DB wrapper
	dbWrapper := &db.DB{DB: testDB}
	repo := NewSourceRepository(dbWrapper)

	// Initially, list should be empty
	sources, err := repo.List()
	if err != nil {
		t.Fatalf("Failed to list sources: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("Expected empty list, got %d sources", len(sources))
	}

	// Create multiple test sources
	testSources := []*models.Source{
		{
			ID:           "list-source-1",
			RawURL:       stringPtrInteg("https://example1.com"),
			ActiveDomain: 1,
			Format:       stringPtrInteg("json"),
			CreatedAt:    time.Now().Add(-2 * time.Hour),
			UpdatedAt:    time.Now().Add(-2 * time.Hour),
		},
		{
			ID:           "list-source-2",
			RawURL:       stringPtrInteg("https://example2.com"),
			ActiveDomain: 1,
			Format:       stringPtrInteg("yaml"),
			CreatedAt:    time.Now().Add(-1 * time.Hour),
			UpdatedAt:    time.Now().Add(-1 * time.Hour),
		},
		{
			ID:           "list-source-3",
			RawURL:       stringPtrInteg("https://example3.com"),
			ActiveDomain: 0,
			Format:       stringPtrInteg("yml"),
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}

	// Create all test sources
	for _, source := range testSources {
		err := repo.Create(source)
		if err != nil {
			t.Fatalf("Failed to create test source %s: %v", source.ID, err)
		}
	}

	// Now list should contain all sources
	sources, err = repo.List()
	if err != nil {
		t.Errorf("Failed to list sources: %v", err)
	}
	if len(sources) != 3 {
		t.Errorf("Expected 3 sources, got %d", len(sources))
	}

	// Verify sources are ordered by created_at DESC (newest first)
	if len(sources) >= 2 {
		if sources[0].CreatedAt.Before(sources[1].CreatedAt) {
			t.Error("Sources should be ordered by created_at DESC")
		}
	}

	// Verify source data
	for _, source := range sources {
		if source.ID == "" {
			t.Error("Source ID should not be empty")
		}
		if source.CreatedAt.IsZero() {
			t.Error("Created at should not be zero")
		}
		if source.UpdatedAt.IsZero() {
			t.Error("Updated at should not be zero")
		}
	}
}

func TestSourceRepository_Update_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	testDB := testutil.SetupTestDB(t)
	defer testutil.CleanupTestDB(t, testDB)

	// Create DB wrapper
	dbWrapper := &db.DB{DB: testDB}
	repo := NewSourceRepository(dbWrapper)

	// Create a test source first
	originalSource := &models.Source{
		ID:           "update-source-test",
		AuthorEmail:  stringPtrInteg("original@example.com"),
		RawURL:       stringPtrInteg("https://original.com"),
		Scheme:       stringPtrInteg("https"),
		Host:         stringPtrInteg("original.com"),
		Path:         stringPtrInteg("/original"),
		ActiveDomain: 1,
		Format:       stringPtrInteg("json"),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := repo.Create(originalSource)
	if err != nil {
		t.Fatalf("Failed to create test source: %v", err)
	}

	tests := []struct {
		name        string
		updateData  *models.Source
		expectError bool
		description string
	}{
		{
			name: "update all fields",
			updateData: &models.Source{
				ID:           "update-source-test",
				AuthorEmail:  stringPtrInteg("updated@example.com"),
				RawURL:       stringPtrInteg("https://updated.com"),
				Scheme:       stringPtrInteg("https"),
				Host:         stringPtrInteg("updated.com"),
				Path:         stringPtrInteg("/updated"),
				Query:        stringPtrInteg("new=param"),
				ActiveDomain: 0,
				Format:       stringPtrInteg("yaml"),
			},
			expectError: false,
			description: "should update all fields successfully",
		},
		{
			name: "update with nil fields",
			updateData: &models.Source{
				ID:           "update-source-test",
				AuthorEmail:  nil,
				RawURL:       nil,
				Scheme:       nil,
				Host:         nil,
				Path:         nil,
				Query:        nil,
				ActiveDomain: 1,
				Format:       nil,
			},
			expectError: false,
			description: "should update with nil fields",
		},
		{
			name: "update nonexistent source",
			updateData: &models.Source{
				ID:           "nonexistent-source",
				ActiveDomain: 1,
			},
			expectError: false, // Update should not error even if source doesn't exist
			description: "should handle update of nonexistent source",
		},
		{
			name: "update with invalid format",
			updateData: &models.Source{
				ID:           "update-source-test",
				ActiveDomain: 1,
				Format:       stringPtrInteg("invalid"),
			},
			expectError: true,
			description: "should fail with invalid format constraint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Add small delay to ensure timestamp changes
			time.Sleep(1 * time.Second)
			err := repo.Update(tt.updateData)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
			}

			// If update was successful and source exists, verify the changes
			if !tt.expectError && err == nil && tt.updateData.ID == "update-source-test" {
				updatedSource, getErr := repo.GetByID(tt.updateData.ID)
				if getErr != nil {
					t.Errorf("Failed to retrieve updated source: %v", getErr)
				} else {
					if updatedSource.ActiveDomain != tt.updateData.ActiveDomain {
						t.Errorf("Expected active domain %d, got %d", tt.updateData.ActiveDomain, updatedSource.ActiveDomain)
					}
					// Verify that updated_at was changed by the database
					if !updatedSource.UpdatedAt.After(originalSource.UpdatedAt) {
						t.Error("UpdatedAt should be automatically updated")
					}
				}
			}
		})
	}
}

func TestSourceRepository_Delete_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	testDB := testutil.SetupTestDB(t)
	defer testutil.CleanupTestDB(t, testDB)

	// Create DB wrapper
	dbWrapper := &db.DB{DB: testDB}
	repo := NewSourceRepository(dbWrapper)

	// Create test sources
	testSources := []*models.Source{
		{
			ID:           "delete-source-1",
			ActiveDomain: 1,
			Format:       stringPtrInteg("json"),
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           "delete-source-2",
			ActiveDomain: 1,
			Format:       stringPtrInteg("yaml"),
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}

	// Create the sources
	for _, source := range testSources {
		err := repo.Create(source)
		if err != nil {
			t.Fatalf("Failed to create test source %s: %v", source.ID, err)
		}
	}

	tests := []struct {
		name        string
		sourceID    string
		expectError bool
		description string
	}{
		{
			name:        "delete existing source",
			sourceID:    "delete-source-1",
			expectError: false,
			description: "should delete existing source successfully",
		},
		{
			name:        "delete nonexistent source",
			sourceID:    "nonexistent-source",
			expectError: false, // Should not error
			description: "should handle deletion of nonexistent source",
		},
		{
			name:        "delete with empty ID",
			sourceID:    "",
			expectError: false, // Should not error but affect 0 rows
			description: "should handle empty source ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check if source exists before deletion
			sourceExistsBefore := false
			if tt.sourceID != "" {
				_, err := repo.GetByID(tt.sourceID)
				sourceExistsBefore = (err == nil)
			}

			err := repo.Delete(tt.sourceID)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
			}

			// If deletion was for an existing source, verify it's gone
			if !tt.expectError && err == nil && sourceExistsBefore {
				_, getErr := repo.GetByID(tt.sourceID)
				if getErr != errSourceNotFound {
					t.Errorf("Expected source to be deleted, but it still exists or got different error: %v", getErr)
				}
			}
		})
	}

	// Verify the remaining source still exists
	_, err := repo.GetByID("delete-source-2")
	if err != nil {
		t.Errorf("Second source should still exist: %v", err)
	}
}

func TestSourceRepository_FullWorkflow_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	testDB := testutil.SetupTestDB(t)
	defer testutil.CleanupTestDB(t, testDB)

	// Create DB wrapper
	dbWrapper := &db.DB{DB: testDB}
	repo := NewSourceRepository(dbWrapper)

	// Test complete CRUD workflow
	source := &models.Source{
		ID:           "workflow-test-source",
		AuthorEmail:  stringPtrInteg("workflow@example.com"),
		RawURL:       stringPtrInteg("https://workflow.com/api"),
		Scheme:       stringPtrInteg("https"),
		Host:         stringPtrInteg("workflow.com"),
		Path:         stringPtrInteg("/api"),
		Query:        stringPtrInteg("version=1"),
		ActiveDomain: 1,
		Format:       stringPtrInteg("json"),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// 1. Create
	err := repo.Create(source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	// 2. Read
	retrievedSource, err := repo.GetByID(source.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve source: %v", err)
	}
	if retrievedSource.ID != source.ID {
		t.Errorf("Retrieved source ID mismatch")
	}

	// 3. List (should contain our source)
	sources, err := repo.List()
	if err != nil {
		t.Fatalf("Failed to list sources: %v", err)
	}
	found := false
	for _, s := range sources {
		if s.ID == source.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Source not found in list")
	}

	// 4. Update
	retrievedSource.AuthorEmail = stringPtrInteg("updated-workflow@example.com")
	retrievedSource.ActiveDomain = 0
	// Add small delay to ensure timestamp changes
	time.Sleep(1 * time.Second)
	err = repo.Update(retrievedSource)
	if err != nil {
		t.Fatalf("Failed to update source: %v", err)
	}

	// Verify update
	updatedSource, err := repo.GetByID(source.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve updated source: %v", err)
	}
	if updatedSource.AuthorEmail == nil || *updatedSource.AuthorEmail != "updated-workflow@example.com" {
		t.Error("Update was not applied correctly")
	}
	if updatedSource.ActiveDomain != 0 {
		t.Error("Active domain update was not applied correctly")
	}

	// 5. Delete
	err = repo.Delete(source.ID)
	if err != nil {
		t.Fatalf("Failed to delete source: %v", err)
	}

	// Verify deletion
	_, err = repo.GetByID(source.ID)
	if err != errSourceNotFound {
		t.Error("Source should have been deleted")
	}
}

// Helper function to create string pointer for integration tests
func stringPtrInteg(s string) *string {
	return &s
}
