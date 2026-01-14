package storage

import (
	"fmt"
	"time"

	"github.com/vmihailenco/msgpack/v5"
	bolt "go.etcd.io/bbolt"
)

var (
	databasesBucket = []byte("databases")
	backupsBucket   = []byte("backups")
	usersBucket     = []byte("users")
	sessionsBucket  = []byte("sessions")
	settingsBucket  = []byte("settings")
)

// BoltStorage implements Storage interface using BoltDB
type BoltStorage struct {
	db      *bolt.DB
	dataDir string
}

// NewBoltStorage creates a new BoltDB-backed storage
func NewBoltStorage(path string, dataDir string) (*BoltStorage, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open bolt database: %w", err)
	}

	// Create buckets
	err = db.Update(func(tx *bolt.Tx) error {
	for _, bucket := range [][]byte{databasesBucket, backupsBucket, usersBucket, sessionsBucket, settingsBucket} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create buckets: %w", err)
	}

	return &BoltStorage{db: db, dataDir: dataDir}, nil
}

// Close closes the database
func (s *BoltStorage) Close() error {
	return s.db.Close()
}

// DataDir returns the data directory
func (s *BoltStorage) DataDir() string {
	return s.dataDir
}

// Database operations

// CreateDatabase stores a new database
func (s *BoltStorage) CreateDatabase(db *DatabaseInstance) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(databasesBucket)
		data, err := msgpack.Marshal(db)
		if err != nil {
			return err
		}
		return b.Put([]byte(db.ID), data)
	})
}

// GetDatabase retrieves a database by ID
func (s *BoltStorage) GetDatabase(id string) (*DatabaseInstance, error) {
	var db DatabaseInstance
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(databasesBucket)
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("database not found: %s", id)
		}
		return msgpack.Unmarshal(data, &db)
	})
	if err != nil {
		return nil, err
	}
	return &db, nil
}

// ListDatabases returns all databases
func (s *BoltStorage) ListDatabases() []*DatabaseInstance {
	var dbs []*DatabaseInstance
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(databasesBucket)
		return b.ForEach(func(k, v []byte) error {
			var db DatabaseInstance
			if err := msgpack.Unmarshal(v, &db); err != nil {
				return err
			}
			dbs = append(dbs, &db)
			return nil
		})
	})
	return dbs
}

// UpdateDatabase updates an existing database
func (s *BoltStorage) UpdateDatabase(db *DatabaseInstance) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(databasesBucket)
		if b.Get([]byte(db.ID)) == nil {
			return fmt.Errorf("database not found: %s", db.ID)
		}
		data, err := msgpack.Marshal(db)
		if err != nil {
			return err
		}
		return b.Put([]byte(db.ID), data)
	})
}

// DeleteDatabase removes a database
func (s *BoltStorage) DeleteDatabase(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(databasesBucket)
		if b.Get([]byte(id)) == nil {
			return fmt.Errorf("database not found: %s", id)
		}
		return b.Delete([]byte(id))
	})
}

// Backup operations

// CreateBackup stores a new backup
func (s *BoltStorage) CreateBackup(backup *Backup) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(backupsBucket)
		data, err := msgpack.Marshal(backup)
		if err != nil {
			return err
		}
		return b.Put([]byte(backup.ID), data)
	})
}

// GetBackup retrieves a backup by ID
func (s *BoltStorage) GetBackup(id string) (*Backup, error) {
	var backup Backup
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(backupsBucket)
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("backup not found: %s", id)
		}
		return msgpack.Unmarshal(data, &backup)
	})
	if err != nil {
		return nil, err
	}
	return &backup, nil
}

// GetBackupPath returns the file path for a backup
func (s *BoltStorage) GetBackupPath(id string) string {
	backup, err := s.GetBackup(id)
	if err != nil {
		return ""
	}
	return backup.FilePath
}

// ListBackups returns all backups, optionally filtered by database ID
func (s *BoltStorage) ListBackups(databaseID string) []*Backup {
	var backups []*Backup
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(backupsBucket)
		return b.ForEach(func(k, v []byte) error {
			var backup Backup
			if err := msgpack.Unmarshal(v, &backup); err != nil {
				return err
			}
			if databaseID == "" || backup.DatabaseID == databaseID {
				backups = append(backups, &backup)
			}
			return nil
		})
	})
	return backups
}

// UpdateBackup updates an existing backup
func (s *BoltStorage) UpdateBackup(backup *Backup) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(backupsBucket)
		if b.Get([]byte(backup.ID)) == nil {
			return fmt.Errorf("backup not found: %s", backup.ID)
		}
		data, err := msgpack.Marshal(backup)
		if err != nil {
			return err
		}
		return b.Put([]byte(backup.ID), data)
	})
}

// DeleteBackup removes a backup
func (s *BoltStorage) DeleteBackup(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(backupsBucket)
		if b.Get([]byte(id)) == nil {
			return fmt.Errorf("backup not found: %s", id)
		}
		return b.Delete([]byte(id))
	})
}

// Settings operations

// GetSetting retrieves a setting value
func (s *BoltStorage) GetSetting(key string) (string, error) {
	var value string
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(settingsBucket)
		data := b.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("setting not found: %s", key)
		}
		value = string(data)
		return nil
	})
	return value, err
}

// SetSetting stores a setting value
func (s *BoltStorage) SetSetting(key, value string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(settingsBucket)
		return b.Put([]byte(key), []byte(value))
	})
}

// User operations

// CreateUser stores a new user
func (s *BoltStorage) CreateUser(user *User) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(usersBucket)
		data, err := msgpack.Marshal(user)
		if err != nil {
			return err
		}
		return b.Put([]byte(user.ID), data)
	})
}

// GetUser retrieves a user by ID
func (s *BoltStorage) GetUser(id string) (*User, error) {
	var user User
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(usersBucket)
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("user not found: %s", id)
		}
		return msgpack.Unmarshal(data, &user)
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (s *BoltStorage) GetUserByUsername(username string) (*User, error) {
	var user *User
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(usersBucket)
		return b.ForEach(func(k, v []byte) error {
			var u User
			if err := msgpack.Unmarshal(v, &u); err != nil {
				return err
			}
			if u.Username == username {
				user = &u
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("user not found: %s", username)
	}
	return user, nil
}

// ListUsers returns all users
func (s *BoltStorage) ListUsers() []*User {
	var users []*User
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(usersBucket)
		return b.ForEach(func(k, v []byte) error {
			var user User
			if err := msgpack.Unmarshal(v, &user); err != nil {
				return err
			}
			users = append(users, &user)
			return nil
		})
	})
	return users
}

// UpdateUser updates an existing user
func (s *BoltStorage) UpdateUser(user *User) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(usersBucket)
		if b.Get([]byte(user.ID)) == nil {
			return fmt.Errorf("user not found: %s", user.ID)
		}
		data, err := msgpack.Marshal(user)
		if err != nil {
			return err
		}
		return b.Put([]byte(user.ID), data)
	})
}

// DeleteUser removes a user
func (s *BoltStorage) DeleteUser(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(usersBucket)
		if b.Get([]byte(id)) == nil {
			return fmt.Errorf("user not found: %s", id)
		}
		return b.Delete([]byte(id))
	})
}

// UserCount returns the number of users
func (s *BoltStorage) UserCount() int {
	var count int
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(usersBucket)
		count = b.Stats().KeyN
		return nil
	})
	return count
}

// Session operations

// CreateSession stores a new session
func (s *BoltStorage) CreateSession(session *Session) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(sessionsBucket)
		data, err := msgpack.Marshal(session)
		if err != nil {
			return err
		}
		return b.Put([]byte(session.ID), data)
	})
}

// GetSession retrieves a session by ID
func (s *BoltStorage) GetSession(id string) (*Session, error) {
	var session Session
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(sessionsBucket)
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("session not found: %s", id)
		}
		return msgpack.Unmarshal(data, &session)
	})
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// GetSessionByToken retrieves a session by token
func (s *BoltStorage) GetSessionByToken(token string) (*Session, error) {
	var session *Session
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(sessionsBucket)
		return b.ForEach(func(k, v []byte) error {
			var s Session
			if err := msgpack.Unmarshal(v, &s); err != nil {
				return err
			}
			if s.Token == token {
				session = &s
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("session not found")
	}
	return session, nil
}

// DeleteSession removes a session
func (s *BoltStorage) DeleteSession(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(sessionsBucket)
		return b.Delete([]byte(id))
	})
}

// DeleteExpiredSessions removes all expired sessions
func (s *BoltStorage) DeleteExpiredSessions() error {
	now := time.Now()
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(sessionsBucket)
		var toDelete [][]byte
		err := b.ForEach(func(k, v []byte) error {
			var session Session
			if err := msgpack.Unmarshal(v, &session); err != nil {
				return nil // skip invalid entries
			}
			if session.ExpiresAt.Before(now) {
				toDelete = append(toDelete, k)
			}
			return nil
		})
		if err != nil {
			return err
		}
		for _, key := range toDelete {
			if err := b.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
}
