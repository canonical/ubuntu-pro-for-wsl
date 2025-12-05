// Package database contains the DistroDB object and its methods. It manages a database
// of Windows Subsystem for Linux distribution instances (aka distros).
package database

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

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/consts"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	"github.com/ubuntu/decorate"
	"go.yaml.in/yaml/v3"
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

	storageDir string

	ctx       context.Context
	cancelCtx func()
	once      sync.Once

	// Multiple distros starting at the same time can cause WSL (and the whole machine) to freeze up.
	// This mutex is used to block multiple distros from starting at the same time.
	distroStartMu sync.Mutex

	onCleanup []func(string)
}

// New creates a database and populates it with data in the file located
// at "storagePath". Changes to the database will be written on this file.
//
// You must call Close to deallocate resources.
//
// Creating multiple databases with the same disk backing will result in
// undefined behaviour.
//
// Every certain amount of times, the database wil purge all distros that
// are no longer registered or that have been marked as unreachable. This
// cleanup can be triggered on demmand with TriggerCleanup.
func New(ctx context.Context, storageDir string, onCleanup ...func(string)) (db *DistroDB, err error) {
	defer decorate.OnError(&err, "could not initialize database")

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	ctx, cancel := context.WithCancel(ctx)

	db = &DistroDB{
		storageDir:      storageDir,
		scheduleTrigger: make(chan struct{}),
		ctx:             ctx,
		cancelCtx:       cancel,
		onCleanup:       onCleanup,
	}

	if err := db.load(ctx); err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case <-db.ctx.Done():
				return
			case <-time.After(timeBetweenGC):
			case <-db.scheduleTrigger:
			}

			if err := db.cleanup(ctx); err != nil {
				log.Errorf(ctx, "Database: failed to clean up potentially unused distros: %v", err)
			}
		}
	}()

	return db, nil
}

// Get searches for the target distro. It returns the distro object and a
// flag indicating if it was found.
// TODO: check if useful as public.
func (db *DistroDB) Get(name string) (distro *distro.Distro, ok bool) {
	if db.stopped() {
		panic("Get: database already stopped")
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	d, ok := db.distros[strings.ToLower(name)]
	return d, ok
}

// GetAll returns a slice with all the distros in the database.
func (db *DistroDB) GetAll() (all []*distro.Distro) {
	if db.stopped() {
		panic("GetAll: database already stopped")
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	for _, v := range db.distros {
		all = append(all, v)
	}

	return all
}

// GetDistroAndUpdateProperties fetches a distro from the database, guranteeing that the
// returned distro is valid, is in the database, and matches the given properties. If needed:
// * A pre-existing distro with the same name may be removed from the database.
// * An existing distro in the database may have their properties updated.
// * A new distro may be added to the database.
func (db *DistroDB) GetDistroAndUpdateProperties(ctx context.Context, name string, props distro.Properties) (*distro.Distro, error) {
	if db.stopped() {
		panic("GetDistroAndUpdateProperties: database already stopped")
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	normalizedName := strings.ToLower(name)
	d, found := db.distros[normalizedName]

	// Name not in database: create a new distro and returns it
	if !found {
		log.Debugf(ctx, "Database: cache miss, creating %q and adding it to the database", name)

		d, err := distro.New(db.ctx, name, props, db.storageDir, &db.distroStartMu)
		if err != nil {
			return nil, err
		}
		db.distros[normalizedName] = d
		err = db.dump()
		return d, err
	}

	// Check that the distro exists and GUId of registered object still matching the one on the system

	// Name in database, wrong GUID: stops previous distro runner and creates a new one.
	if !d.IsValid() {
		log.Debugf(ctx, "Database: cache overwrite. Distro %q removed and added again", name)

		go d.Cleanup(ctx)
		delete(db.distros, normalizedName)

		d, err := distro.New(db.ctx, name, props, db.storageDir, &db.distroStartMu)
		if err != nil {
			return nil, err
		}
		db.distros[normalizedName] = d
		err = db.dump()
		return d, err
	}

	log.Debugf(ctx, "Database: cache hit. Overwriting properties for %q", name)

	// Name in database, correct GUID: refresh with latest properties of a valid distro
	var err error
	if d.SetProperties(props) {
		err = db.dump()
	}

	return d, err
}

// Dump stores the current database state to disk, overriding old dumps.
// Next time we start the agent, the database will be loaded from this dump.
func (db *DistroDB) Dump() error {
	if db.stopped() {
		panic("Dump: database already stopped")
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	return db.dump()
}

// TriggerCleanup forces the database cleanup loop to skip its current delay and
// call autoCleanup immediately. It is blocking until the cleanup starts.
func (db *DistroDB) TriggerCleanup() {
	if db.stopped() {
		panic("TriggerCleanup: database already stopped")
	}

	db.scheduleTrigger <- struct{}{}
}

// cleanup removes any distro that no longer exists or has been reset from the database.
func (db *DistroDB) cleanup(ctx context.Context) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	var needsDBDump bool
	for name, d := range db.distros {
		if d.IsValid() {
			continue
		}

		log.Infof(ctx, "Database: distro %q became invalid, cleaning up.", d.Name())
		for _, f := range db.onCleanup {
			if f != nil {
				f(name)
			}
		}
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
func (db *DistroDB) load(ctx context.Context) (err error) {
	defer decorate.OnError(&err, "failed to load database from disk")

	// Read raw database from disk
	out, err := os.ReadFile(filepath.Join(db.storageDir, consts.DatabaseFileName))
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
		return fmt.Errorf("could not unmarshal: %v", err)
	}

	// Initializing distros into database
	db.distros = make(map[string]*distro.Distro, len(distros))
	for _, inert := range distros {
		d, err := inert.newDistro(ctx, db.storageDir, &db.distroStartMu)
		if err != nil {
			log.Warningf(ctx, "Database: read invalid distro from database: %#+v", inert)
			continue
		}
		db.distros[strings.ToLower(d.Name())] = d
	}

	return nil
}

// dump writes the database contents into the file inside db.storageDir.
// The dump is deterministic, with distros always sorted alphabetically.
func (db *DistroDB) dump() (err error) {
	defer decorate.OnError(&err, "failed to dump database to disk")

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
		return fmt.Errorf("could not marshal: %v", err)
	}

	// Write dump
	storagePath := filepath.Join(db.storageDir, consts.DatabaseFileName)
	err = os.WriteFile(storagePath+".new", out, 0600)
	if err != nil {
		return err
	}

	err = os.Rename(storagePath+".new", storagePath)
	if err != nil {
		return err
	}

	return nil
}

func (db *DistroDB) stopped() bool {
	select {
	case <-db.ctx.Done():
		return true
	default:
		return false
	}
}

// Close frees up resources allocated to database maintenance and
// ensures the database contents are written to file.
func (db *DistroDB) Close(ctx context.Context) {
	db.once.Do(func() {
		db.cancelCtx()

		if err := db.cleanup(ctx); err != nil {
			log.Warningf(ctx, "Database: error while closing: %v", err)
		}

		if err := db.dump(); err != nil {
			log.Warningf(ctx, "Database: error while closing: %v", err)
		}

		close(db.scheduleTrigger)
		db.cleanupAllDistros(ctx)
	})
}

// cleanupAllDistros signals all distro task processing goroutines to stop
// and blocks until all of them have done so.
func (db *DistroDB) cleanupAllDistros(ctx context.Context) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var wg sync.WaitGroup
	for _, d := range db.distros {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.Cleanup(ctx)
		}()
	}

	wg.Wait()

	// Leave behind an empty map to avoid operating on stopped distros
	db.distros = map[string]*distro.Distro{}
}
