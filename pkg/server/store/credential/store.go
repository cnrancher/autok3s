package credential

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types/apis"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
)

type Store struct {
	empty.Store
}

func (cred *Store) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	credID, err := strconv.Atoi(id)
	if err != nil {
		return types.APIObject{}, apierror.NewAPIError(validation.InvalidOption, fmt.Sprintf("invalid id %s", id))
	}
	c, err := common.DefaultDB.GetCredential(credID)
	if err != nil {
		return types.APIObject{}, err
	}
	if c == nil {
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, fmt.Sprintf("credential %s is not exist", id))
	}

	credential, err := toCredential(c)
	if err != nil {
		return types.APIObject{}, err
	}
	return types.APIObject{
		Type:   schema.ID,
		ID:     strconv.Itoa(c.ID),
		Object: credential,
	}, nil
}

func (cred *Store) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	credList, err := common.DefaultDB.ListCredential()
	if err != nil {
		return types.APIObjectList{}, err
	}
	result := types.APIObjectList{}
	for _, c := range credList {
		credential, err := toCredential(c)
		if err != nil {
			logrus.Errorf("failed to convert credential secrets to map: %v", err)
			continue
		}
		result.Objects = append(result.Objects, types.APIObject{
			Type:   schema.ID,
			ID:     strconv.Itoa(c.ID),
			Object: credential,
		})
	}
	return result, nil
}

func (cred *Store) Create(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject) (types.APIObject, error) {
	secrets := data.Data().Map("secrets")
	p := data.Data().String("provider")
	c, err := generateCredential(secrets, p)
	if err != nil {
		return types.APIObject{}, err
	}
	err = common.DefaultDB.CreateCredential(c)
	if err != nil {
		return types.APIObject{}, err
	}
	credential, err := toCredential(c)
	if err != nil {
		return types.APIObject{}, err
	}
	return types.APIObject{
		Type:   schema.ID,
		ID:     strconv.Itoa(c.ID),
		Object: credential,
	}, nil
}

func (cred *Store) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	credID, err := strconv.Atoi(id)
	if err != nil {
		return types.APIObject{}, apierror.NewAPIError(validation.InvalidOption, fmt.Sprintf("invalid id %s", id))
	}
	secrets := data.Data().Map("secrets")
	p := data.Data().String("provider")
	c, err := generateCredential(secrets, p)
	if err != nil {
		return types.APIObject{}, err
	}
	c.ID = credID
	err = common.DefaultDB.UpdateCredential(c)
	if err != nil {
		return types.APIObject{}, err
	}
	return cred.ByID(apiOp, schema, id)
}

func (cred *Store) Delete(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	credID, err := strconv.Atoi(id)
	if err != nil {
		return types.APIObject{}, apierror.NewAPIError(validation.InvalidOption, fmt.Sprintf("invalid id %s", id))
	}
	err = common.DefaultDB.DeleteCredential(credID)
	return types.APIObject{}, err
}

func generateCredential(secrets map[string]interface{}, p string) (*common.Credential, error) {
	provider, err := providers.GetProvider(p)
	if err != nil {
		return nil, apierror.NewAPIError(validation.NotFound, err.Error())
	}
	// valid credential keys
	flags := provider.GetCredentialFlags()
	for _, f := range flags {
		if _, ok := secrets[f.Name]; !ok {
			return nil, apierror.NewAPIError(validation.InvalidOption, fmt.Sprintf("missing credential %s", f.Name))
		}
	}
	value, err := json.Marshal(secrets)
	if err != nil {
		return nil, err
	}
	c := &common.Credential{
		Provider: p,
		Secrets:  value,
	}
	return c, nil
}

func toCredential(c *common.Credential) (*apis.Credential, error) {
	secrets := map[string]string{}
	err := json.Unmarshal(c.Secrets, &secrets)
	if err != nil {
		return nil, err
	}
	credential := &apis.Credential{
		ID:       c.ID,
		Provider: c.Provider,
		Secrets:  secrets,
	}
	return credential, nil
}
