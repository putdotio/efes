package tablecache

import (
	"database/sql"
	"sync"
	"time"

	"github.com/cenkalti/log"
	"github.com/putdotio/efes/logger"
)

// Class represents a record in "class" table.
type Class struct {
	ID        int
	Name      string
	Replicate int
}

// Host represents a record in "host" table.
type Host struct {
	ID      int
	Name    string
	LanIP   string
	Devices []Device
}

// Device represents a record in "device" table.
type Device struct {
	ID            int
	Port          int
	ReceiverPort  int
	Status        string
	TotalSpace    int64
	FreeSpace     int64
	IOUtilization int
}

// TableCache keeps a cache of records from "class", "host" and "device" tables in the database.
type TableCache struct {
	sync.RWMutex
	db            *sql.DB
	log           *logger.Logger
	Interval      time.Duration
	ClassesByName map[string]Class
	ClassesByID   map[int]*Class
	HostsByID     map[int]Host
	DeviceByID    map[int]*Device
}

// New returns a new TableCache instance.
// User must call Do() first to make initial cache then call Run in a separate goroutine to keep the cache up to date.
func New(db *sql.DB, logger *logger.Logger) *TableCache {
	return &TableCache{
		db:            db,
		log:           logger,
		Interval:      time.Second,
		ClassesByName: make(map[string]Class),
		HostsByID:     make(map[int]Host),
	}
}

// Do runs queries against database to update tables in cache.
func (c *TableCache) Do() error {
	ClassesByName := make(map[string]Class)
	ClassesByID := make(map[int]*Class)
	HostsByID := make(map[int]Host)
	DeviceByID := make(map[int]*Device)

	// Query all tables concurrently
	var wg sync.WaitGroup
	wg.Add(2)
	var errChan = make(chan error, 2)
	go fetchClasses(c.db, &wg, ClassesByName, ClassesByID, errChan)
	go fetchHosts(c.db, &wg, HostsByID, DeviceByID, errChan)
	// TODO Timeouts are unhandled. If a query does not return, cache is not going to be updated.
	wg.Wait()

	select {
	case err := <-errChan:
		return err
	default:
	}

	// Lock time is very short
	c.Lock()
	c.ClassesByName = ClassesByName
	c.HostsByID = HostsByID
	c.DeviceByID = DeviceByID
	c.ClassesByID = ClassesByID
	c.Unlock()
	return nil
}

// Run keeps the cache up to date by running Do periodically.
func (c *TableCache) Run(stop <-chan struct{}, done *sync.WaitGroup) {
	defer done.Done()

	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.log.Debug("Updating table cache")
			err := c.Do()
			if err != nil {
				c.log.Errorln("Error while updating table cache:", err)
			}
		case <-stop:
			return
		}
	}
}

// fetchClasses retrieves class records from the database.
func fetchClasses(db *sql.DB, wg *sync.WaitGroup, ClassesByName map[string]Class, ClassesByID map[int]*Class, errChan chan error) {
	defer wg.Done()
	rows, err := db.Query("SELECT * FROM class")
	if err != nil {
		errChan <- err
		return
	}
	defer func() {
		if err = rows.Close(); err != nil {
			log.Errorf("Error while closing query: %s", err)
		}
	}()

	for rows.Next() {
		var c Class
		err = rows.Scan(&c.ID, &c.Name, &c.Replicate)
		if err != nil {
			errChan <- err
			return
		}
		ClassesByName[c.Name] = c
		ClassesByID[c.ID] = &c
	}
	if err = rows.Err(); err != nil {
		errChan <- err
		return
	}
}

// fetchHosts retrieves alive devices and its hosts from the database.
func fetchHosts(db *sql.DB, wg *sync.WaitGroup, HostsByID map[int]Host, DeviceByID map[int]*Device, errChan chan error) {
	defer wg.Done()
	var totalSpace sql.NullInt64
	var freeSpace sql.NullInt64
	var ioUtil sql.NullInt64

	rows, err := db.Query("SELECT host.id, host.name, host.lan_ip, device.id, device.port, device.receiver_port, " +
		"device.total_space, device.free_space, device.io_utilization, device.status " +
		"FROM host RIGHT JOIN device ON(host.id = device.host_id)")
	if err != nil {
		errChan <- err
		return
	}
	defer func() {
		if err = rows.Close(); err != nil {
			log.Errorf("Error while closing query: %s", err)
		}
	}()

	for rows.Next() {
		var h Host
		var d Device
		err = rows.Scan(&h.ID, &h.Name, &h.LanIP, &d.ID, &d.Port, &d.ReceiverPort, &totalSpace,
			&freeSpace, &ioUtil, &d.Status)
		if err != nil {
			errChan <- err
			return
		}

		if existingHost, ok := HostsByID[h.ID]; ok {
			existingHost.Devices = append(existingHost.Devices, d)
			HostsByID[h.ID] = existingHost
		} else {
			h.Devices = append(h.Devices, d)
			HostsByID[h.ID] = h
		}

		if ioUtil.Valid {
			ts, er := ioUtil.Value()
			if er != nil {
				continue
			}
			d.IOUtilization = int(ts.(int64))
		}

		if freeSpace.Valid {
			fs, er := freeSpace.Value()
			if er != nil {
				continue
			}
			d.FreeSpace = fs.(int64)
		}

		if totalSpace.Valid {
			s, er := totalSpace.Value()
			if er != nil {
				continue
			}
			d.TotalSpace = s.(int64)
		}
		DeviceByID[d.ID] = &d
	}
	if err = rows.Err(); err != nil {
		errChan <- err
		return
	}
}
