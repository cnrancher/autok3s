package common

import (
	"path/filepath"

	"github.com/cnrancher/autok3s/pkg/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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
				is_default               bool,
				unique (name, provider)
			);`,
		`CREATE TABLE IF NOT EXISTS credentials
			(
				id integer not null primary key autoincrement,
				provider TEXT not null,
				secrets BLOB
			);`,
	}
)

func InitStorage() error {
	if err := utils.EnsureFileExist(filepath.Join(CfgPath, DBFolder), DBFile); err != nil {
		return err
	}
	dataSource := GetDataSource()
	db, err := gorm.Open(sqlite.Open(dataSource), &gorm.Config{})
	if err != nil {
		return err
	}
	setup(db)
	return db.AutoMigrate(&ClusterState{}, &Template{})
}

func setup(db *gorm.DB) {
	for _, statement := range schema {
		db.Exec(statement)
	}
}

func GetDB() (*gorm.DB, error) {
	dataSource := GetDataSource()
	return gorm.Open(sqlite.Open(dataSource), &gorm.Config{})
}
