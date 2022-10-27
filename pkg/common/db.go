package common

import (
	"context"

	"github.com/cnrancher/autok3s/pkg/settings"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	schema = []string{
		`CREATE TABLE IF NOT EXISTS cluster_states
			(
				name                     TEXT not null,
				provider                 TEXT not null,
				token                    TEXT,
				ip                       TEXT,
				tls_sans                 TEXT,
				cluster_cidr             TEXT,
				master_extra_args        TEXT,
				worker_extra_args        TEXT,
				registry                 TEXT,
				registry_content         TEXT,
				data_store               TEXT,
				k3s_version              TEXT,
				k3s_channel              TEXT,
				install_script           TEXT,
				mirror                   TEXT,
				docker_mirror            TEXT,
				docker_script            TEXT,
				network                  TEXT,
				ui                       bool,
				cluster                  bool,
				options                  BLOB,
				status                   TEXT,
				master_nodes             BLOB,
				worker_nodes             BLOB,
				context_name             TEXT,
				master                   TEXT,
				worker                   TEXT,
				ssh_port                 TEXT,
				ssh_user                 TEXT,
				ssh_password             TEXT,
				ssh_key_path             TEXT,
				ssh_cert                 TEXT,
				ssh_cert_path            TEXT,
				ssh_key_passphrase       TEXT,
				ssh_agent_auth           bool,
				manifests                TEXT,
				enable                   TEXT,
				standalone               bool,
				unique (name, provider)
			);`,
		`CREATE TABLE IF NOT EXISTS templates
			(
				context_name             TEXT,
				name                     TEXT not null,
				provider                 TEXT not null,
				token                    TEXT,
				ip                       TEXT,
				tls_sans                 TEXT,
				cluster_cidr             TEXT,
				master_extra_args        TEXT,
				worker_extra_args        TEXT,
				registry                 TEXT,
                registry_content         TEXT,
				data_store               TEXT,
				k3s_version              TEXT,
				k3s_channel              TEXT,
				install_script           TEXT,
				mirror                   TEXT,
				docker_mirror            TEXT,
				docker_script            TEXT,
				network                  TEXT,
				ui                       bool,
				cluster                  bool,
				options                  BLOB,
				master                   TEXT,
				worker                   TEXT,
				ssh_port                 TEXT,
				ssh_user                 TEXT,
				ssh_password             TEXT,
				ssh_key_path             TEXT,
				ssh_cert                 TEXT,
				ssh_cert_path            TEXT,
				ssh_key_passphrase       TEXT,
				ssh_agent_auth           bool,
				manifests                TEXT,
				enable                   TEXT,
				is_default               bool,
				unique (name, provider)
			);`,
	}
)

// InitStorage initializes database storage.
func InitStorage(ctx context.Context) error {
	dataSource := GetDataSource()
	if err := utils.EnsureFileExist(dataSource); err != nil {
		return err
	}

	store, err := NewClusterDB(ctx)
	if err != nil {
		return err
	}

	setup(store.DB)
	if err := store.DB.AutoMigrate(
		&ClusterState{},
		&Template{},
		&Package{},
		// TODO
		// Migrate Credential table will cause create table error when upgrading autok3s from 0.5.x
		// So we are not migrating this table for now and needs a better upgrade solution in later version.
		// &Credential{},
		&Explorer{},
		&Setting{},
	); err != nil {
		return err
	}

	DefaultDB = store

	return settings.SetProvider(&DBSettingProvider{})
}

// GetDB open and returns database.
func GetDB() (*gorm.DB, error) {
	dataSource := GetDataSource()
	config := &gorm.Config{}
	if IsCLI && !Debug {
		config.Logger = logger.Default.LogMode(logger.Silent)
	}
	return gorm.Open(sqlite.Open(dataSource), config)
}

func setup(db *gorm.DB) {
	for _, statement := range schema {
		db.Exec(statement)
	}
}
