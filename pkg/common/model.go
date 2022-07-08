package common

import (
	"context"
	"encoding/json"

	"github.com/cnrancher/autok3s/pkg/metrics"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/apis"

	apitypes "github.com/rancher/apiserver/pkg/types"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ClusterState cluster state struct.
type ClusterState struct {
	types.Metadata `json:",inline" mapstructure:",squash" gorm:"embedded"`
	Options        []byte `json:"options,omitempty" gorm:"type:bytes"`
	Status         string `json:"status" yaml:"status"`
	Standalone     bool   `json:"standalone" yaml:"standalone" gorm:"type:bool"`
	MasterNodes    []byte `json:"master-nodes,omitempty" gorm:"type:bytes"`
	WorkerNodes    []byte `json:"worker-nodes,omitempty" gorm:"type:bytes"`
	types.SSH      `json:",inline" mapstructure:",squash" gorm:"embedded"`
}

// Template template struct.
type Template struct {
	types.Metadata `json:",inline" mapstructure:",squash" gorm:"embedded"`
	Options        []byte `json:"options,omitempty" gorm:"type:bytes"`
	types.SSH      `json:",inline" mapstructure:",squash" gorm:"embedded"`
	IsDefault      bool `json:"is-default" gorm:"type:bool"`
}

// Credential credential struct.
type Credential struct {
	ID       int    `json:"id" gorm:"type:integer"`
	Provider string `json:"provider"`
	Secrets  []byte `json:"secrets,omitempty" gorm:"type:bytes"`
}

// Explorer struct
type Explorer struct {
	ContextName string `json:"context-name"`
	Enabled     bool   `json:"enabled"`
	Port        int    `json:"port"`
}

// Setting struct
type Setting struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type templateEvent struct {
	Name   string
	Object *Template
}

type clusterEvent struct {
	Name   string
	Object *ClusterState
}

// LogEvent log event struct.
type LogEvent struct {
	Name        string
	ContextName string
}

type explorerEvent struct {
	Name   string
	Object *Explorer
}

// Store holds broadcaster's API state.
type Store struct {
	*gorm.DB
	broadcaster *Broadcaster
}

// NewClusterDB new cluster store.
func NewClusterDB(ctx context.Context) (*Store, error) {
	gormDB, err := GetDB()
	if err != nil {
		return nil, err
	}
	gormDB.WithContext(ctx)

	// Fix: SQLite "database is locked (5) (SQLITE_BUSY)".
	// Fix: https://github.com/cnrancher/autok3s/issues/460.
	db, err := gormDB.DB()
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	return &Store{
		DB:          gormDB,
		broadcaster: NewBroadcaster(),
	}, nil
}

// Register register gorm create/update hook.
func (d *Store) Register() {
	_ = d.DB.Callback().Create().After("gorm:create").Register("gorm:autok3s_create", d.createHandler)
	_ = d.DB.Callback().Update().After("gorm:update").Register("gorm:autok3s_update", d.updateHandler)
}

func (d *Store) createHandler(db *gorm.DB) {
	d.hook(db, apitypes.CreateAPIEvent)
}

func (d *Store) updateHandler(db *gorm.DB) {
	d.hook(db, apitypes.ChangeAPIEvent)
}

func (d *Store) hook(db *gorm.DB, event string) {
	// TODO refactor for common function
	if db.Statement.Schema != nil {
		if db.Statement.Schema.Name == "Template" {
			temp := convertModelToTemplate(db.Statement.Model)
			if temp != nil {
				d.broadcaster.Broadcast(&templateEvent{
					Name:   event,
					Object: temp,
				})
			}
		} else if db.Statement.Schema.Name == "ClusterState" {
			state := convertToClusterState(db.Statement.Model)
			if state != nil {
				d.broadcaster.Broadcast(&clusterEvent{
					Name:   event,
					Object: state,
				})
			}
		} else if db.Statement.Schema.Name == "Explorer" {
			exp := convertToExplorer(db.Statement.Model)
			if exp != nil {
				d.broadcaster.Broadcast(&explorerEvent{
					Name:   event,
					Object: exp,
				})
			}
		}
	}
}

// BroadcastObject broadcast object.
func (d *Store) BroadcastObject(obj interface{}) {
	d.broadcaster.Broadcast(obj)
}

// WatchCluster watch cluster.
func (d *Store) WatchCluster(apiOp *apitypes.APIRequest, schema *apitypes.APISchema, input chan apitypes.APIEvent) {
	// new subscribe.
	sub := d.broadcaster.Register(func(v interface{}) bool {
		_, ok := v.(*clusterEvent)
		return ok
	})
	for {
		select {
		case v, ok := <-sub:
			if !ok {
				continue
			}
			state, isCluster := v.(*clusterEvent)
			if !isCluster {
				continue
			}
			input <- toClusterEvent(state, schema.ID)
		case <-apiOp.Context().Done():
			d.broadcaster.Evict(sub)
			return
		}
	}
}

// Log subscribe log.
func (d *Store) Log(apiOp *apitypes.APIRequest, input chan *LogEvent) {
	// new subscribe for cluster logs
	sub := d.broadcaster.Register(func(v interface{}) bool {
		_, ok := v.(*LogEvent)
		return ok
	})
	for {
		select {
		case v, ok := <-sub:
			if !ok {
				continue
			}
			state, isLog := v.(*LogEvent)
			if !isLog {
				continue
			}
			input <- state
		case <-apiOp.Context().Done():
			d.broadcaster.Evict(sub)
			return
		}
	}
}

// Explorer watch K3s cluster setting changes for kube-explorer
func (d *Store) Explorer(apiOp *apitypes.APIRequest, schema *apitypes.APISchema, input chan apitypes.APIEvent) {
	sub := d.broadcaster.Register(func(v interface{}) bool {
		_, ok := v.(*explorerEvent)
		return ok
	})
	for {
		select {
		case v, ok := <-sub:
			if !ok {
				continue
			}
			exp, isExplorer := v.(*explorerEvent)
			if !isExplorer {
				continue
			}
			input <- toExplorerEvent(exp, schema.ID)
		case <-apiOp.Context().Done():
			d.broadcaster.Evict(sub)
			return
		}
	}
}

func toClusterEvent(state *clusterEvent, schemaID string) apitypes.APIEvent {
	return apitypes.APIEvent{
		Name:         state.Name,
		ResourceType: schemaID,
		Object: apitypes.APIObject{
			Type:   schemaID,
			ID:     state.Object.ContextName,
			Object: ConvertToCluster(state.Object),
		},
	}
}

func ConvertToCluster(state *ClusterState) types.Cluster {
	c := types.Cluster{
		Metadata: state.Metadata,
		SSH:      state.SSH,
		Status: types.Status{
			Status: state.Status,
		},
	}

	p, err := providers.GetProvider(state.Provider)
	if err != nil {
		logrus.Errorf("failed to get provider by name %s", state.Provider)
		return c
	}
	opt, err := p.GetProviderOptions(state.Options)
	if err != nil {
		logrus.Errorf("failed to convert [%s] provider options %s: %v", state.Provider, string(state.Options), err)
		return c
	}
	c.Options = opt
	return c
}

func convertToClusterState(m interface{}) *ClusterState {
	if m == nil {
		return nil
	}
	model, err := json.Marshal(m)
	if err != nil {
		logrus.Errorf("failed to convert model %v to bytes: %v", m, err)
		return nil
	}
	state := &ClusterState{}
	err = json.Unmarshal(model, state)
	if err != nil {
		logrus.Errorf("failed to convert model %v to cluster state: %v", m, err)
		return nil
	}
	return state
}

func convertModelToTemplate(m interface{}) *Template {
	if m == nil {
		return nil
	}
	model, err := json.Marshal(m)
	if err != nil {
		logrus.Errorf("failed to convert model %v to bytes: %v", m, err)
		return nil
	}
	temp := &Template{}
	err = json.Unmarshal(model, temp)
	if err != nil {
		logrus.Errorf("failed to convert model %v to template: %v", m, err)
		return nil
	}
	return temp
}

func convertToExplorer(m interface{}) *Explorer {
	if m == nil {
		return nil
	}
	model, err := json.Marshal(m)
	if err != nil {
		logrus.Errorf("failed to convert model %v to bytes: %v", m, err)
		return nil
	}
	exp := &Explorer{}
	err = json.Unmarshal(model, exp)
	if err != nil {
		logrus.Errorf("failed to convert model %v to explorer: %v", m, err)
		return nil
	}
	return exp
}

func toExplorerEvent(event *explorerEvent, schemaID string) apitypes.APIEvent {
	return apitypes.APIEvent{
		Name:         event.Name,
		ResourceType: schemaID,
		Object: apitypes.APIObject{
			Type:   schemaID,
			ID:     event.Object.ContextName,
			Object: event.Object,
		},
	}
}

// SaveCluster save cluster.
func (d *Store) SaveCluster(cluster *types.Cluster) error {
	// find cluster
	state := &ClusterState{}
	result := d.DB.Where("name = ? AND provider = ?", cluster.Name, cluster.Provider).Find(state)

	opt, err := json.Marshal(cluster.Options)
	if err != nil {
		return err
	}
	masterNodeBytes, err := json.Marshal(cluster.Status.MasterNodes)
	if err != nil {
		return err
	}
	workerNodeBytes, err := json.Marshal(cluster.Status.WorkerNodes)
	if err != nil {
		return err
	}
	state = &ClusterState{
		Metadata:    cluster.Metadata,
		Options:     opt,
		Status:      cluster.Status.Status,
		MasterNodes: masterNodeBytes,
		WorkerNodes: workerNodeBytes,
		SSH:         cluster.SSH,
		Standalone:  cluster.Status.Standalone,
	}

	if result.RowsAffected == 0 {
		// create cluster
		result = d.DB.Create(state)
		if result.Error == nil {
			metrics.ClusterCount.With(getLabelsFromMeta(state.Metadata)).Inc()
		}
		return result.Error
	}
	result = d.DB.Model(state).
		Where("name = ? AND provider = ?", cluster.Name, cluster.Provider).
		Omit("name", "provider", "context_name").Save(state)
	return result.Error
}

// SaveClusterState save cluster state.
func (d *Store) SaveClusterState(state *ClusterState) error {
	result := d.DB.Model(state).
		Where("name = ? AND provider = ?", state.Name, state.Provider).
		Omit("name", "provider").Save(state)
	return result.Error
}

// DeleteCluster delete cluster.
func (d *Store) DeleteCluster(name, provider string) error {
	state, err := d.GetCluster(name, provider)
	if err != nil {
		return err
	}
	if state == nil {
		return nil
	}
	result := d.DB.Where("name = ? AND provider = ?", name, provider).Delete(&ClusterState{})
	d.broadcaster.Broadcast(&clusterEvent{
		Name:   apitypes.RemoveAPIEvent,
		Object: state,
	})
	if result.Error == nil {
		metrics.ClusterCount.With(getLabelsFromMeta(state.Metadata)).Dec()
	}
	return result.Error
}

// ListCluster list cluster.
func (d *Store) ListCluster(provider string) ([]*ClusterState, error) {
	clusterList := make([]*ClusterState, 0)
	db := d.DB
	if provider != "" {
		db = db.Where("provider = ?", provider)
	}
	result := db.Find(&clusterList)
	return clusterList, result.Error
}

// GetCluster get cluster.
func (d *Store) GetCluster(name, provider string) (*ClusterState, error) {
	state := &ClusterState{}
	result := d.DB.Where("name = ? AND provider = ?", name, provider).Find(state)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return state, nil
}

// GetClusterByID get cluster by ID.
func (d *Store) GetClusterByID(contextName string) (*ClusterState, error) {
	state := &ClusterState{}
	result := d.DB.Where("context_name = ? ", contextName).Find(state)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return state, nil
}

// FindCluster find cluster.
func (d *Store) FindCluster(name, provider string) ([]*ClusterState, error) {
	clusterList := make([]*ClusterState, 0)
	db := d.DB.Where("name = ?", name)
	if provider != "" {
		db = db.Where("provider = ?", provider)
	}
	result := db.Find(&clusterList)
	return clusterList, result.Error
}

// CreateTemplate create template.
func (d *Store) CreateTemplate(template *Template) error {
	result := d.DB.Create(template)
	if result.Error == nil {
		metrics.TemplateCount.With(getLabelsFromMeta(template.Metadata)).Inc()
	}
	return result.Error
}

// UpdateTemplate update template.
func (d *Store) UpdateTemplate(template *Template) error {
	result := d.DB.Model(template).
		Where("name = ? AND provider = ?", template.Name, template.Provider).
		Omit("name", "provider", "context_name").Save(template)
	return result.Error
}

// DeleteTemplate delete template.
func (d *Store) DeleteTemplate(name, provider string) error {
	temp, err := d.GetTemplate(name, provider)
	if err != nil {
		return err
	}
	result := d.DB.
		Where("name = ? AND provider = ?", name, provider).
		Delete(&Template{})
	if result.Error == nil {
		d.broadcaster.Broadcast(&templateEvent{
			Name:   apitypes.RemoveAPIEvent,
			Object: temp,
		})
		metrics.TemplateCount.With(getLabelsFromMeta(temp.Metadata)).Dec()
	}
	return result.Error
}

// ListTemplates list template.
func (d *Store) ListTemplates() ([]*Template, error) {
	list := make([]*Template, 0)
	result := d.DB.Find(&list)
	return list, result.Error
}

// GetTemplate get template.
func (d *Store) GetTemplate(name, provider string) (*Template, error) {
	template := &Template{}
	result := d.DB.Where("name = ? AND provider = ?", name, provider).
		Find(template)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return template, nil
}

// WatchTemplate watch template.
func (d *Store) WatchTemplate(apiOp *apitypes.APIRequest, schema *apitypes.APISchema, input chan apitypes.APIEvent) {
	// new subscribe for template
	sub := d.broadcaster.Register(func(v interface{}) bool {
		_, ok := v.(*templateEvent)
		return ok
	})
	for {
		select {
		case v, ok := <-sub:
			if !ok {
				continue
			}
			temp, isTemplate := v.(*templateEvent)
			if !isTemplate {
				continue
			}
			input <- toTemplateEvent(temp, schema.ID)
		case <-apiOp.Context().Done():
			d.broadcaster.Evict(sub)
			return
		}
	}
}

func toTemplateEvent(state *templateEvent, schemaID string) apitypes.APIEvent {
	return apitypes.APIEvent{
		Name:         state.Name,
		ResourceType: schemaID,
		Object: apitypes.APIObject{
			Type:   schemaID,
			ID:     state.Object.ContextName,
			Object: toTemplate(state.Object),
		},
	}
}

func toTemplate(temp *Template) *apis.ClusterTemplate {
	c := &apis.ClusterTemplate{
		Metadata:  temp.Metadata,
		SSH:       temp.SSH,
		IsDefault: temp.IsDefault,
	}
	p, err := providers.GetProvider(temp.Provider)
	if err != nil {
		logrus.Errorf("failed to get provider by name %s", temp.Provider)
		return c
	}
	opt, err := p.GetProviderOptions(temp.Options)
	if err != nil {
		logrus.Errorf("failed to convert [%s] provider options %s: %v", temp.Provider, string(temp.Options), err)
		return c
	}
	c.Options = opt
	return c
}

// CreateCredential create credential.
func (d *Store) CreateCredential(cred *Credential) error {
	// find exist provider credential.
	list, err := d.GetCredentialByProvider(cred.Provider)
	if err != nil {
		return err
	}
	if len(list) > 0 {
		// TODO: need to support multiple credentials for each provider.
		logrus.Warnf("only support one credential for provider %s, will update with the new one.", cred.Provider)
		credential := list[0]
		credential.Secrets = cred.Secrets
		result := d.DB.Updates(credential)
		return result.Error
	}
	result := d.DB.Create(cred)
	return result.Error
}

// UpdateCredential update credential.
func (d *Store) UpdateCredential(cred *Credential) error {
	result := d.DB.Model(cred).
		Where("id = ? ", cred.ID).
		Omit("id", "provider").Save(cred)
	return result.Error
}

// ListCredential list credential.
func (d *Store) ListCredential() ([]*Credential, error) {
	list := make([]*Credential, 0)
	result := d.DB.Find(&list)
	return list, result.Error
}

// GetCredentialByProvider get credential by provider.
func (d *Store) GetCredentialByProvider(provider string) ([]*Credential, error) {
	list := make([]*Credential, 0)
	result := d.DB.Where("provider = ? ", provider).Find(&list)
	return list, result.Error
}

// GetCredential get credential by ID.
func (d *Store) GetCredential(id int) (*Credential, error) {
	cred := &Credential{}
	result := d.DB.Where("id = ? ", id).Find(cred)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return cred, nil
}

// DeleteCredential delete credential by ID.
func (d *Store) DeleteCredential(id int) error {
	cred := &Credential{}
	result := d.DB.Where("id = ? ", id).Delete(cred)
	return result.Error
}

// SaveExplorer save cluster kube-explorer settings
func (d *Store) SaveExplorer(exp *Explorer) error {
	e, err := d.GetExplorer(exp.ContextName)
	if err != nil {
		return err
	}
	if e != nil {
		// update explorer setting
		result := d.DB.Where("context_name = ? ", exp.ContextName).Omit("context_name").Save(exp)
		return result.Error
	}
	// save explorer setting
	result := d.DB.Create(exp)
	return result.Error
}

// GetExplorer return explorer setting for specified cluster
func (d *Store) GetExplorer(clusterID string) (*Explorer, error) {
	exp := &Explorer{}
	result := d.DB.Where("context_name = ? ", clusterID).Find(exp)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return exp, nil
}

// DeleteExplorer remove explorer setting for specified cluster
func (d *Store) DeleteExplorer(clusterID string) error {
	result := d.DB.Where("context_name = ? ", clusterID).Delete(&Explorer{})
	return result.Error
}

// ListExplorer return all kube-explorer settings
func (d *Store) ListExplorer() ([]*Explorer, error) {
	list := make([]*Explorer, 0)
	result := d.DB.Find(&list)
	return list, result.Error
}

// GetSetting return specified setting by name
func (d *Store) GetSetting(name string) (*Setting, error) {
	s := &Setting{}
	result := d.DB.Where("name = ?", name).Find(s)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return s, nil
}

// SaveSetting save settings
func (d *Store) SaveSetting(s *Setting) error {
	e, err := d.GetSetting(s.Name)
	if err != nil {
		return err
	}
	if e != nil {
		// update setting
		result := d.DB.Where("name = ? ", s.Name).Omit("name").Save(s)
		return result.Error
	}
	// save setting
	result := d.DB.Create(s)
	return result.Error
}

// ListSettings list all settings
func (d *Store) ListSettings() ([]*Setting, error) {
	list := make([]*Setting, 0)
	result := d.DB.Find(&list)
	return list, result.Error
}
