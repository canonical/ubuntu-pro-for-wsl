// Package distroDB contains the DistroDB object and its methods. It manages a database
// of Windows Subsystem for Linux distribution instances (aka distros).
package distroDB

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/consts"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distro"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"gopkg.in/yaml.v3"
)

const (
	timeBetweenGC = time.Hour
)

// DistroDB is a thread-safe single-table database of WSL distribution instances. This
// database is held in memory and backed in disk. Any write on the database will be instanly
// followed up by a write-to-disk.
type DistroDB struct {
	distros map[string]*distro.Distro
	mu      sync.RWMutex

	scheduleTrigger chan struct{}

	storagePath string
}

// New creates a database and populates it with data in the file located
// at "storagePath". Changes to the database will be written on this file.
//
// Creating multiple databases with the same disk backing will result in
// undefined behaviour.
// TODO: write about the auto gc.
func New(storageDir string) (*DistroDB, error) {
	if err := os.MkdirAll(storageDir, 0600); err != nil {
		return nil, fmt.Errorf("could not create database directory: %w", err)
	}

	db := &DistroDB{
		storagePath:     filepath.Join(storageDir, consts.DatabaseFileName),
		scheduleTrigger: make(chan struct{}),
	}
	if err := db.load(); err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case <-time.After(timeBetweenGC):
			case <-db.scheduleTrigger:
			}
			if err := db.autoCleanup(context.TODO()); err != nil {
				log.Errorf(context.TODO(), "Failed to clean up potentially unused distros: %v", err)
			}
		}
	}()

	return db, nil
}

// Get searches for the target distro. It returns the distro object and a
// flag indicating if it was found.
// TODO: check if useful as public.
func (db *DistroDB) Get(name string) (distro *distro.Distro, ok bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	d, ok := db.distros[strings.ToLower(name)]
	return d, ok
}

// GetDistroAndUpdateProperties fetches a distro from the database, guranteeing that the
// returned distro is valid, is in the database, and matches the given properties. If needed:
// * A pre-existing distro with the same name may be removed from the database.
// * An existing distro in the database may have their properties updated.
// * A new distro may be added to the database.
func (db *DistroDB) GetDistroAndUpdateProperties(ctx context.Context, name string, props distro.Properties) (*distro.Distro, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	normalizedName := strings.ToLower(name)
	d, found := db.distros[normalizedName]

	// Name not in database: create a new distro and returns it
	if !found {
		log.Debugf(ctx, "Cache miss, creating %q and adding it to the database", name)

		d, err := distro.New(name, props)
		if err != nil {
			return nil, err
		}
		db.distros[normalizedName] = d
		err = db.dump()
		return d, err
	}

	// Check that the distro exists and GUId of registered object still matching the one on the system
	isvalid, err := d.IsValid()
	if err != nil {
		return nil, err
	}

	// Name in database, wrong GUID: stops previous distro runner and creates a new one.
	if !isvalid {
		log.Debugf(ctx, "Cache overwrite. Distro %q removed and added again", name)

		go d.Cleanup(context.TODO())
		delete(db.distros, normalizedName)

		d, err := distro.New(name, props)
		if err != nil {
			return nil, err
		}
		db.distros[normalizedName] = d
		err = db.dump()
		return d, err
	}

	log.Debugf(ctx, "Cache hit. Overwriting properties for %q", name)

	// Name in database, correct GUID: refresh with latest properties of a valid distro
	err = nil
	if d.Properties != props {
		d.Properties = props
		err = db.dump()
	}

	return d, err
}

// Dump stores the current database state to disk, overriding old dumps.
// Next time we start the agent, the database will be loaded from this dump.
func (db *DistroDB) Dump() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	return db.dump()
}

// TriggerCleanup forces the database cleanup loop to skip its current delay and
// call autoCleanup immediately. It is blocking until the cleanup starts.
func (db *DistroDB) TriggerCleanup() {
	db.scheduleTrigger <- struct{}{}
}

// autoCleanup removes any distro that no longer exists or has been reset from the database.
func (db *DistroDB) autoCleanup(ctx context.Context) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	var needsDBDump bool
	for name, d := range db.distros {
		if d.UnreachableErr == nil {
			continue
		}

		log.Infof(ctx, "Distro %q became invalid, cleaning up: %v", name, d.UnreachableErr)
		go d.Cleanup(ctx)
		delete(db.distros, name)
		needsDBDump = true
	}
	if needsDBDump {
		return db.dump()
	}
	return nil
}

// load reads the database from disk.
func (db *DistroDB) load() error {
	// Read raw database from disk
	out, err := os.ReadFile(db.storagePath)
	if errors.Is(err, fs.ErrNotExist) {
		db.distros = make(map[string]*distro.Distro)
		return nil
	}
	if err != nil {
		return err
	}

	// Parse database into intermediate objects
	distros := make([]serializableDistro, 0)
	err = yaml.Unmarshal(out, &distros)
	if err != nil {
		return err
	}

	// Initializing distros into database
	db.distros = make(map[string]*distro.Distro, len(distros))
	for _, inert := range distros {
		d, err := inert.newDistro()
		if err != nil {
			log.Warningf(context.TODO(), "Read invalid distro from database: %#+v", inert)
			continue
		}
		db.distros[strings.ToLower(d.Name)] = d
	}

	return nil
}

// dump writes the database contents into the file specified by db.storagePath.
// The dump is deterministic, with distros always sorted alphabetically.
func (db *DistroDB) dump() error {
	// Sort distros case-independently.
	normalizedNames := make([]string, 0, len(db.distros))
	for n := range db.distros {
		normalizedNames = append(normalizedNames, n)
	}
	sort.Strings(normalizedNames)

	// Create intermediate easy-to-marshall objects
	distros := make([]serializableDistro, 0, len(db.distros))
	for _, n := range normalizedNames {
		distros = append(distros, newSerializableDistro(db.distros[n]))
	}

	// Generate dump
	out, err := yaml.Marshal(distros)
	if err != nil {
		return err
	}

	// Write dump
	err = os.WriteFile(db.storagePath+".new", out, 0600)
	if err != nil {
		return err
	}

	err = os.Rename(db.storagePath+".new", db.storagePath)
	if err != nil {
		return err
	}

	return nil
}
