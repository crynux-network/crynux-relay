package migrate

import (
	"crynux_relay/migrate/migrations"

	"github.com/go-gormigrate/gormigrate/v2"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var migrationScripts []*gormigrate.Gormigrate

func Migrate() error {
	for _, migrationScript := range migrationScripts {
		if err := migrationScript.Migrate(); err != nil {
			log.Errorf("Migrate: %v", err)
			return err
		}
	}

	return nil
}

func Rollback() error {
	lastMigration := migrationScripts[len(migrationScripts)-1]

	if err := lastMigration.RollbackLast(); err != nil {
		log.Errorf("Migrate: %v", err)
		return err
	}

	return nil
}

func InitMigration(db *gorm.DB) {
	db.Set("gorm:table_options", "CHARSET=utf8mb4")

	// Add new migrations here
	migrationScripts = append(migrationScripts, migrations.M20230810(db))
	migrationScripts = append(migrationScripts, migrations.M20230824(db))
	migrationScripts = append(migrationScripts, migrations.M20240115(db))
	migrationScripts = append(migrationScripts, migrations.M20240518(db))
	migrationScripts = append(migrationScripts, migrations.M20240522(db))
	migrationScripts = append(migrationScripts, migrations.M20240530(db))
	migrationScripts = append(migrationScripts, migrations.M20240613(db))
	migrationScripts = append(migrationScripts, migrations.M20240717(db))
	migrationScripts = append(migrationScripts, migrations.M20240924(db))
	migrationScripts = append(migrationScripts, migrations.M20240925(db))
	migrationScripts = append(migrationScripts, migrations.M20240925_1(db))
	migrationScripts = append(migrationScripts, migrations.M20240925_2(db))
	migrationScripts = append(migrationScripts, migrations.M20240925_3(db))
	migrationScripts = append(migrationScripts, migrations.M20240927(db))
	migrationScripts = append(migrationScripts, migrations.M20240929(db))
	migrationScripts = append(migrationScripts, migrations.M20241009(db))
	migrationScripts = append(migrationScripts, migrations.M20241011(db))
	migrationScripts = append(migrationScripts, migrations.M20241012(db))
	migrationScripts = append(migrationScripts, migrations.M20241015(db))
	migrationScripts = append(migrationScripts, migrations.M20241204(db))
	migrationScripts = append(migrationScripts, migrations.M20250313(db))
	migrationScripts = append(migrationScripts, migrations.M20250402(db))
	migrationScripts = append(migrationScripts, migrations.M20250417(db))
	migrationScripts = append(migrationScripts, migrations.M20250422(db))
	migrationScripts = append(migrationScripts, migrations.M20250422_1(db))
	migrationScripts = append(migrationScripts, migrations.M20250427(db))
	migrationScripts = append(migrationScripts, migrations.M20250428(db))
	migrationScripts = append(migrationScripts, migrations.M20250715(db))
	migrationScripts = append(migrationScripts, migrations.M20250725(db))
	migrationScripts = append(migrationScripts, migrations.M20250728(db))
	migrationScripts = append(migrationScripts, migrations.M20250821(db))
	migrationScripts = append(migrationScripts, migrations.M20250826(db))
	migrationScripts = append(migrationScripts, migrations.M20250902(db))
	migrationScripts = append(migrationScripts, migrations.M20250904(db))
	migrationScripts = append(migrationScripts, migrations.M20250905(db))
	migrationScripts = append(migrationScripts, migrations.M20250906(db))
	migrationScripts = append(migrationScripts, migrations.M20250925(db))
	migrationScripts = append(migrationScripts, migrations.M20251001(db))
	migrationScripts = append(migrationScripts, migrations.M20251014(db))
	migrationScripts = append(migrationScripts, migrations.M20251019(db))
	migrationScripts = append(migrationScripts, migrations.M20251023(db))
	migrationScripts = append(migrationScripts, migrations.M20251106(db))
	migrationScripts = append(migrationScripts, migrations.M20251110(db))
	migrationScripts = append(migrationScripts, migrations.M20251121(db))
}
