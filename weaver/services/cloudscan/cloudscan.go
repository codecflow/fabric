package cloudscan

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/codecflow/fabric/pkg/dto"
)

type (
	ScanProvider interface {
		Scan(ctx context.Context) ([]dto.Machine, error)
	}

	ScanService struct {
		db       *sql.DB
		interval time.Duration
		scanners []ScanProvider
	}
)

func NewScanService(db *sql.DB, interval time.Duration, scanners ...ScanProvider) *ScanService {
	return &ScanService{db: db, interval: interval, scanners: scanners}
}

func (svc *ScanService) Run(ctx context.Context) error {
	ticker := time.NewTicker(svc.interval)
	defer ticker.Stop()

	if err := svc.scanAll(ctx); err != nil {
		log.Printf("initial scan failed: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := svc.scanAll(ctx); err != nil {
				log.Printf("scan failed: %v", err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (svc *ScanService) scanAll(ctx context.Context) error {
	for _, scanner := range svc.scanners {
		machines, err := scanner.Scan(ctx)
		if err != nil {
			log.Printf("scanner %T error: %v", scanner, err)
			continue
		}
		if err := svc.saveMachines(ctx, machines); err != nil {
			log.Printf("saveMachines error: %v", err)
		}
	}
	return nil
}

func (svc *ScanService) saveMachines(ctx context.Context, machines []dto.Machine) error {
	tx, err := svc.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
        INSERT OR REPLACE INTO machines 
          (id, name, cpu, memory_mb, region, price, timestamp)
        VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, m := range machines {
		if _, err := stmt.ExecContext(ctx,
			m.ID, m.Name, m.CPU, m.MemoryMB, m.Region, m.Price, m.Timestamp,
		); err != nil {
			log.Printf("failed to upsert machine %s: %v", m.ID, err)
		}
	}
	return tx.Commit()
}
