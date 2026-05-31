package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Entry struct {
	BackupID       string    `json:"backup_id"`
	ChainID        string    `json:"chain_id"`
	DBType         string    `json:"db_type"`
	Database       string    `json:"database"`
	Mode           string    `json:"mode"`
	ArtifactKind   string    `json:"artifact_kind"`
	ArtifactURI    string    `json:"artifact_uri"`
	BasisBackupID  string    `json:"basis_backup_id,omitempty"`
	ParentBackupID string    `json:"parent_backup_id,omitempty"`
	Sequence       int       `json:"sequence"`
	Compression    string    `json:"compression"`
	Checksum       string    `json:"checksum"`
	SizeBytes      int64     `json:"size_bytes"`
	CreatedAt      time.Time `json:"created_at"`
}

type Catalog struct {
	Entries []Entry `json:"entries"`
}

func Load(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Catalog{Entries: []Entry{}}, nil
		}
		return nil, fmt.Errorf("failed to read catalog: %w", err)
	}

	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("failed to decode catalog: %w", err)
	}
	if c.Entries == nil {
		c.Entries = []Entry{}
	}
	return &c, nil
}

func (c *Catalog) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create catalog directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode catalog: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("failed to write catalog temp file: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("failed to commit catalog: %w", err)
	}
	return nil
}

func (c *Catalog) Add(entry Entry) {
	c.Entries = append(c.Entries, entry)
	sort.SliceStable(c.Entries, func(i, j int) bool {
		if c.Entries[i].CreatedAt.Equal(c.Entries[j].CreatedAt) {
			return c.Entries[i].Sequence < c.Entries[j].Sequence
		}
		return c.Entries[i].CreatedAt.Before(c.Entries[j].CreatedAt)
	})
}

func (c *Catalog) FindByBackupID(backupID string) (*Entry, bool) {
	for i := range c.Entries {
		if c.Entries[i].BackupID == backupID {
			return &c.Entries[i], true
		}
	}
	return nil, false
}

func (c *Catalog) FindByURI(uri string) (*Entry, bool) {
	for i := range c.Entries {
		if c.Entries[i].ArtifactURI == uri {
			return &c.Entries[i], true
		}
	}
	return nil, false
}

func (c *Catalog) LatestForDatabase(dbType, database string) (*Entry, bool) {
	var latest *Entry
	for i := range c.Entries {
		e := &c.Entries[i]
		if strings.EqualFold(e.DBType, dbType) && e.Database == database {
			if latest == nil || latest.CreatedAt.Before(e.CreatedAt) || (latest.CreatedAt.Equal(e.CreatedAt) && latest.Sequence < e.Sequence) {
				latest = e
			}
		}
	}
	return latest, latest != nil
}

func (c *Catalog) LatestFullForDatabase(dbType, database string) (*Entry, bool) {
	var latest *Entry
	for i := range c.Entries {
		e := &c.Entries[i]
		if strings.EqualFold(e.DBType, dbType) && e.Database == database && strings.EqualFold(e.Mode, "full") {
			if latest == nil || latest.CreatedAt.Before(e.CreatedAt) || (latest.CreatedAt.Equal(e.CreatedAt) && latest.Sequence < e.Sequence) {
				latest = e
			}
		}
	}
	return latest, latest != nil
}

func (c *Catalog) ChainTo(target *Entry) ([]Entry, error) {
	if target == nil {
		return nil, errors.New("target entry is nil")
	}

	var chain []Entry
	seen := map[string]bool{}
	current := target
	for current != nil {
		if seen[current.BackupID] {
			return nil, fmt.Errorf("catalog chain loop detected at %s", current.BackupID)
		}
		seen[current.BackupID] = true
		chain = append(chain, *current)
		if current.ParentBackupID == "" {
			break
		}
		next, ok := c.FindByBackupID(current.ParentBackupID)
		if !ok {
			return nil, fmt.Errorf("catalog missing parent backup %s", current.ParentBackupID)
		}
		current = next
	}

	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}
