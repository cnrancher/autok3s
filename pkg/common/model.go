package common

import (
	"context"
	"encoding/json"

	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/apis"

	apitypes "github.com/rancher/apiserver/pkg/types"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ClusterState struct {
	types.Metadata `json:",inline" mapstructure:",squash" gorm:"embedded"`
	Options        []byte `json:"options,omitempty" gorm:"type:bytes"`
	Status         string `json:"status" yaml:"status"`
	MasterNodes    []byte `json:"master-nodes,omitempty" gorm:"type:bytes"`
	WorkerNodes    []byte `json:"worker-nodes,omitempty" gorm:"type:bytes"`
	types.SSH      `json:",inline" mapstructure:",squash" gorm:"embedded"`
}

type Template struct {
	types.Metadata `json:",inline" mapstructure:",squash" gorm:"embedded"`
	Options        []byte `json:"options,omitempty" gorm:"type:bytes"`
	types.SSH      `json:",inline" mapstructure:",squash" gorm:"embedded"`
	IsDefault      bool `json:"is-default" gorm:"type:bool"`
}

type Credential struct {
	ID       int    `json:"id" gorm:"type:integer"`
	Provider string `json:"provider"`
	Secrets  []byte `json:"secrets,omitempty" gorm:"type:bytes"`
}

type templateEvent struct {
	Name   string
	Object *Template
}

type clusterEvent struct {
	Name   string
	Object *ClusterState
}

type LogEvent struct {
	Name        string
	ContextName string
}

type Store struct {
	*gorm.DB
	broadcaster *Broadcaster
}

func NewClusterDB(ctx context.Context) (*Store, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}
	db.WithContext(ctx)
	return &Store{
		DB:          db,
		broadcaster: NewBroadcaster(),
	}, nil
}

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
		}
	}
}

func (d *Store) BroadcastObject(obj interface{}) {
	d.broadcaster.Broadcast(obj)
}

func (d *Store) WatchCluster(apiOp *apitypes.APIRequest, schema *apitypes.APISchema, input chan apitypes.APIEvent) {
	// new subscribe
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

func toClusterEvent(state *clusterEvent, schemaID string) apitypes.APIEvent {
	return apitypes.APIEvent{
		Name:         state.Name,
		ResourceType: schemaID,
		Object: apitypes.APIObject{
			Type:   schemaID,
			ID:     state.Object.ContextName,
			Object: toCluster(state.Object),
		},
	}
}

func toCluster(state *ClusterState) types.Cluster {
	c := types.Cluster{
		Metadata: state.Metadata,
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
	}

	if result.RowsAffected == 0 {
		// create cluster
		result = d.DB.Create(state)
		return result.Error
	}
	result = d.DB.Model(state).
		Where("name = ? AND provider = ?", cluster.Name, cluster.Provider).
		Omit("name", "provider", "context_name").Save(state)
	return result.Error
}

func (d *Store) SaveClusterState(state *ClusterState) error {
	result := d.DB.Model(state).
		Where("name = ? AND provider = ?", state.Name, state.Provider).
		Omit("name", "provider").Save(state)
	return result.Error
}

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
	return result.Error
}

func (d *Store) ListCluster() ([]*ClusterState, error) {
	clusterList := make([]*ClusterState, 0)
	result := d.DB.Find(&clusterList)
	return clusterList, result.Error
}

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

func (d *Store) FindCluster(name, provider string) ([]*ClusterState, error) {
	clusterList := make([]*ClusterState, 0)
	db := d.DB.Where("name = ?", name)
	if provider != "" {
		db = db.Where("provider = ?", provider)
	}
	result := db.Find(&clusterList)
	return clusterList, result.Error
}

func (d *Store) CreateTemplate(template *Template) error {
	result := d.DB.Create(template)
	return result.Error
}

func (d *Store) UpdateTemplate(template *Template) error {
	result := d.DB.Model(template).
		Where("name = ? AND provider = ?", template.Name, template.Provider).
		Omit("name", "provider", "context_name").Save(template)
	return result.Error
}

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
	}
	return result.Error
}

func (d *Store) ListTemplates() ([]*Template, error) {
	list := make([]*Template, 0)
	result := d.DB.Find(&list)
	return list, result.Error
}

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

func (d *Store) UpdateCredential(cred *Credential) error {
	result := d.DB.Model(cred).
		Where("id = ? ", cred.ID).
		Omit("id", "provider").Save(cred)
	return result.Error
}

func (d *Store) ListCredential() ([]*Credential, error) {
	list := make([]*Credential, 0)
	result := d.DB.Find(&list)
	return list, result.Error
}

func (d *Store) GetCredentialByProvider(provider string) ([]*Credential, error) {
	list := make([]*Credential, 0)
	result := d.DB.Where("provider = ? ", provider).Find(&list)
	return list, result.Error
}

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

func (d *Store) DeleteCredential(id int) error {
	cred := &Credential{}
	result := d.DB.Where("id = ? ", id).Delete(cred)
	return result.Error
}
